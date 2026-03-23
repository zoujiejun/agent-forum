package service

import (
	"errors"
	"math"
	"regexp"
	"strings"
	"time"

	"github.com/zoujiejun/agent-forum/internal/feishu"
	"github.com/zoujiejun/agent-forum/internal/model"
	"github.com/zoujiejun/agent-forum/internal/repository"
	"gorm.io/gorm"
)

var (
	ErrMemberNotFound = errors.New("member not found")
	ErrTopicNotFound  = errors.New("topic not found")
	ErrTopicClosed    = errors.New("topic is closed")
	ErrUnauthorized   = errors.New("unauthorized")

	mentionNamePattern      = regexp.MustCompile(`@([\p{Han}\p{L}\p{N}_-]+)`)
	boldMentionPattern      = regexp.MustCompile(`\*\*@([\p{Han}\p{L}\p{N}_-]+)\*\*`)
	underlineMentionPattern = regexp.MustCompile(`__@([\p{Han}\p{L}\p{N}_-]+)__`)
	codeMentionPattern      = regexp.MustCompile("`@([\\p{Han}\\p{L}\\p{N}_-]+)`")
)

type Service struct {
	repo           *repository.Repository
	broker         Broker
	feishuNotifier *feishu.Notifier
}

// Broker is the interface for broadcasting WebSocket events (same as websocket.Broker).
type Broker interface {
	BroadcastTopicUpdate(topicID int64, topic interface{})
	BroadcastReplyCreated(topicID int64, reply interface{})
	BroadcastTopicCreated(topic interface{})
	BroadcastTopicClosed(topicID int64)
	SendNotification(agentName string, notification interface{})
}

// SetBroker sets the event broker for WebSocket broadcasting.
func (s *Service) SetBroker(broker Broker) {
	s.broker = broker
}

// SetFeishuNotifier sets the Feishu notifier for sending notifications.
func (s *Service) SetFeishuNotifier(notifier *feishu.Notifier) {
	s.feishuNotifier = notifier
}

func (s *Service) maybeBroadcast(fn func()) {
	if s.broker != nil {
		fn()
	}
}

func New(repo *repository.Repository) *Service {
	return &Service{repo: repo}
}

// RegisterMember registers a member, or returns the existing one if already present.
func (s *Service) RegisterMember(name, workspace string) (*model.Member, error) {
	existing, err := s.repo.GetMemberByName(name)
	if err == nil && existing != nil {
		return existing, nil
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	member := &model.Member{
		Name:      strings.TrimSpace(name),
		Workspace: strings.TrimSpace(workspace),
		CreatedAt: time.Now(),
	}
	if err := s.repo.CreateMember(member); err != nil {
		return nil, err
	}
	return member, nil
}

// GetMember retrieves a member by name.
func (s *Service) GetMember(name string) (*model.Member, error) {
	member, err := s.repo.GetMemberByName(strings.TrimSpace(name))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrMemberNotFound
		}
		return nil, err
	}
	return member, nil
}

// ListMembers lists all members.
func (s *Service) ListMembers() ([]model.Member, error) {
	return s.repo.ListMembers()
}

// CreateTopic creates a topic and generates mention notifications.
func (s *Service) CreateTopic(creatorName, title, content string, mentions []string, tags []string) (*model.Topic, error) {
	creator, err := s.GetMember(creatorName)
	if err != nil {
		return nil, err
	}

	topic := &model.Topic{
		Title:     strings.TrimSpace(title),
		Content:   strings.TrimSpace(content),
		CreatorID: creator.ID,
		Status:    "open",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := s.repo.CreateTopic(topic); err != nil {
		return nil, err
	}

	// Apply initial tags
	if len(tags) > 0 {
		if err := s.repo.SetTopicTags(topic.ID, tags); err != nil {
			return nil, err
		}
	}

	mentionedMemberIDs := make(map[int64]struct{})
	for _, name := range mentions {
		cleanName := strings.TrimSpace(strings.TrimPrefix(name, "@"))
		if cleanName == "" {
			continue
		}

		member, err := s.repo.GetMemberByName(cleanName)
		if err != nil {
			continue
		}
		if member.ID == creator.ID {
			continue
		}
		if _, exists := mentionedMemberIDs[member.ID]; exists {
			continue
		}
		mentionedMemberIDs[member.ID] = struct{}{}

		tm := &model.TopicMention{
			TopicID:     topic.ID,
			MemberID:    member.ID,
			MentionedAt: time.Now(),
		}
		if err := s.repo.AddTopicMention(tm); err != nil {
			return nil, err
		}

		notif := &model.Notification{
			MemberID:  member.ID,
			Type:      "mention",
			TargetID:  topic.ID,
			TopicID:   topic.ID,
			Read:      false,
			CreatedAt: time.Now(),
		}
		if err := s.repo.CreateNotification(notif); err != nil {
			return nil, err
		}

		// Send via WebSocket if broker is available
		s.maybeBroadcast(func() {
			s.broker.SendNotification(cleanName, notif)
		})
	}

	createdTopic, err := s.repo.GetTopic(topic.ID)
	if err != nil {
		return nil, err
	}

	return createdTopic, nil
}

// GetTopic returns a topic with its replies.
func (s *Service) GetTopic(id int64) (*model.TopicWithReplies, error) {
	topic, err := s.repo.GetTopicWithReplies(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTopicNotFound
		}
		return nil, err
	}
	return topic, nil
}

// ListTopics lists topics with pagination.
func (s *Service) ListTopics(page, limit int, status string) ([]model.Topic, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 20
	}
	status = strings.TrimSpace(status)
	topics, err := s.repo.ListTopics(page, limit, status)
	if err != nil {
		return nil, 0, err
	}
	count, err := s.repo.CountTopics(status)
	if err != nil {
		return nil, 0, err
	}
	return topics, count, nil
}

// CloseTopic allows only the topic creator to close the topic.
func (s *Service) CloseTopic(id int64, operatorName string) error {
	topic, err := s.repo.GetTopicByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrTopicNotFound
		}
		return err
	}

	operator, err := s.GetMember(operatorName)
	if err != nil {
		return err
	}
	if topic.CreatorID != operator.ID {
		return ErrUnauthorized
	}

	if err := s.repo.CloseTopic(id); err != nil {
		return err
	}

	return nil
}

// CreateReply creates a reply and notifies relevant participants.
func (s *Service) CreateReply(topicID int64, authorName, content string, replyToID *int64) ([]model.Reply, error) {
	topic, err := s.repo.GetTopicByID(topicID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTopicNotFound
		}
		return nil, err
	}
	if topic.Status == "closed" {
		return nil, ErrTopicClosed
	}

	author, err := s.GetMember(authorName)
	if err != nil {
		return nil, err
	}

	reply := &model.Reply{
		TopicID:   topicID,
		AuthorID:  author.ID,
		Content:   strings.TrimSpace(content),
		ReplyToID: replyToID,
		CreatedAt: time.Now(),
	}

	if err := s.repo.CreateReply(reply); err != nil {
		return nil, err
	}

	// Automatically mark the current user's unread mention notifications for this topic as read
	notifications, err := s.repo.GetNotificationsByTopic(author.ID, topicID)
	if err == nil {
		var mentionIDs []int64
		for _, n := range notifications {
			if !n.Read && n.Type == "mention" {
				mentionIDs = append(mentionIDs, n.ID)
			}
		}
		if len(mentionIDs) > 0 {
			_ = s.repo.MarkNotificationsRead(author.ID, mentionIDs)
		}
	}

	if err := s.notifyReplyMentions(topicID, author, reply.Content); err != nil {
		return nil, err
	}

	if err := s.notifyReplies(topicID, author); err != nil {
		return nil, err
	}

	replies, err := s.repo.GetRepliesByTopic(topicID)
	if err != nil {
		return nil, err
	}

	return replies, nil
}

func normalizeMentionContent(content string) string {
	content = boldMentionPattern.ReplaceAllString(content, "@$1")
	content = underlineMentionPattern.ReplaceAllString(content, "@$1")
	content = codeMentionPattern.ReplaceAllString(content, "@$1")
	return content
}

func extractMentionNames(content string) []string {
	content = normalizeMentionContent(content)
	matches := mentionNamePattern.FindAllStringSubmatch(content, -1)
	seen := make(map[string]struct{})
	var names []string
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		name := strings.TrimSpace(match[1])
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}
	return names
}

func mergeMentionNames(groups ...[]string) []string {
	seen := make(map[string]struct{})
	var names []string
	for _, group := range groups {
		for _, raw := range group {
			name := strings.TrimSpace(strings.TrimPrefix(raw, "@"))
			if name == "" {
				continue
			}
			if _, ok := seen[name]; ok {
				continue
			}
			seen[name] = struct{}{}
			names = append(names, name)
		}
	}
	return names
}

func (s *Service) notifyReplyMentions(topicID int64, author *model.Member, content string) error {
	mentionNames := extractMentionNames(content)
	if len(mentionNames) == 0 {
		return nil
	}

	existingMentions, err := s.repo.GetTopicMentions(topicID)
	if err != nil {
		return err
	}
	existingMentionedMemberIDs := make(map[int64]struct{}, len(existingMentions))
	for _, m := range existingMentions {
		existingMentionedMemberIDs[m.MemberID] = struct{}{}
	}

	notified := make(map[int64]struct{})
	for _, name := range mentionNames {
		member, err := s.repo.GetMemberByName(name)
		if err != nil {
			continue
		}
		if member.ID == author.ID {
			continue
		}
		if _, ok := notified[member.ID]; ok {
			continue
		}
		notified[member.ID] = struct{}{}

		if _, exists := existingMentionedMemberIDs[member.ID]; !exists {
			tm := &model.TopicMention{
				TopicID:     topicID,
				MemberID:    member.ID,
				MentionedAt: time.Now(),
			}
			if err := s.repo.AddTopicMention(tm); err != nil {
				return err
			}
			existingMentionedMemberIDs[member.ID] = struct{}{}
		}

		notif := &model.Notification{
			MemberID:  member.ID,
			Type:      "mention",
			TargetID:  topicID,
			TopicID:   topicID,
			Read:      false,
			CreatedAt: time.Now(),
		}
		if err := s.repo.CreateNotification(notif); err != nil {
			return err
		}

		// Send via WebSocket if broker is available
		s.maybeBroadcast(func() {
			s.broker.SendNotification(name, notif)
		})
	}

	return nil
}

func (s *Service) notifyReplies(topicID int64, author *model.Member) error {
	topic, err := s.repo.GetTopicByID(topicID)
	if err != nil {
		return err
	}

	notified := make(map[int64]struct{})
	createNotification := func(memberID int64) error {
		if memberID == author.ID {
			return nil
		}
		if _, exists := notified[memberID]; exists {
			return nil
		}
		notified[memberID] = struct{}{}

		notif := &model.Notification{
			MemberID:  memberID,
			Type:      "reply",
			TargetID:  topicID,
			TopicID:   topicID,
			Read:      false,
			CreatedAt: time.Now(),
		}
		if err := s.repo.CreateNotification(notif); err != nil {
			return err
		}

		// Send via WebSocket if broker is available
		s.maybeBroadcast(func() {
			member, _ := s.repo.GetMemberByID(memberID)
			if member != nil {
				s.broker.SendNotification(member.Name, notif)
			}
		})
		return nil
	}

	if err := createNotification(topic.CreatorID); err != nil {
		return err
	}

	mentions, err := s.repo.GetTopicMentions(topicID)
	if err != nil {
		return err
	}
	for _, m := range mentions {
		if err := createNotification(m.MemberID); err != nil {
			return err
		}
	}

	return nil
}

// GetNotifications returns unread notifications for a member.
func (s *Service) GetNotifications(memberName string) ([]model.Notification, error) {
	member, err := s.GetMember(memberName)
	if err != nil {
		return nil, err
	}
	return s.repo.GetUnreadNotifications(member.ID)
}

// MarkNotificationsRead marks notifications as read.
func (s *Service) MarkNotificationsRead(memberName string, ids []int64) error {
	member, err := s.GetMember(memberName)
	if err != nil {
		return err
	}
	return s.repo.MarkNotificationsRead(member.ID, ids)
}

// GetMentionedTopics returns topics that still have unread mention notifications for the member.
func (s *Service) GetMentionedTopics(memberName string) ([]model.Topic, error) {
	member, err := s.GetMember(memberName)
	if err != nil {
		return nil, err
	}
	return s.repo.GetMentionedTopics(member.ID)
}

// GetUnreadMentionCount returns the unread mention count.
func (s *Service) GetUnreadMentionCount(memberName string) (int64, error) {
	member, err := s.GetMember(memberName)
	if err != nil {
		return 0, err
	}
	return s.repo.CountUnreadMentionNotifications(member.ID)
}

// UpdateMemberWorkspace updates a member workspace.
func (s *Service) UpdateMemberWorkspace(id int64, workspace string) error {
	return s.repo.UpdateMemberWorkspace(id, workspace)
}

// --- Hotness ---

// HotnessConfig holds weights for the hotness algorithm.
type HotnessConfig struct {
	ReplyWeight   float64 // points per reply
	LikeWeight    float64 // points per like
	ViewWeight    float64 // points per view
	HalfLifeHours float64 // decay half-life in hours
}

var DefaultHotnessConfig = HotnessConfig{
	ReplyWeight:   5.0,
	LikeWeight:    3.0,
	ViewWeight:    0.1,
	HalfLifeHours: 24.0,
}

// CalculateHotness computes the hotness score for a topic.
// Formula: (replies*replyWeight + likes*likeWeight + views*viewWeight) * timeDecay
// timeDecay = 1 / (1 + hoursSinceCreation / halfLifeHours)
func (s *Service) CalculateHotness(topicID int64, replyCount, likeCount, viewCount int, createdAt time.Time) (float64, error) {
	cfg := DefaultHotnessConfig
	hoursSince := time.Since(createdAt).Hours()
	if hoursSince < 0 {
		hoursSince = 0
	}
	rawScore := float64(replyCount)*cfg.ReplyWeight +
		float64(likeCount)*cfg.LikeWeight +
		float64(viewCount)*cfg.ViewWeight

	timeDecay := 1.0 / (1.0 + hoursSince/cfg.HalfLifeHours)
	return math.Round(rawScore*timeDecay*100) / 100, nil
}

// RecalculateTopicHotness recalculates and stores topic hotness.
func (s *Service) RecalculateTopicHotness(topicID int64) error {
	topic, err := s.repo.GetTopicByID(topicID)
	if err != nil {
		return err
	}

	replies, err := s.repo.GetRepliesByTopic(topicID)
	if err != nil {
		return err
	}

	h, err := s.repo.GetTopicHotness(topicID)
	if err != nil {
		return err
	}
	if h == nil {
		h = &model.TopicHotness{TopicID: topicID}
	}

	score, err := s.CalculateHotness(topicID, len(replies), int(h.LikeCount), int(h.ViewCount), topic.CreatedAt)
	if err != nil {
		return err
	}
	h.Hotness = score

	return s.repo.UpsertTopicHotness(h)
}

// RefreshAllHotness recalculates hotness for all topics.
func (s *Service) RefreshAllHotness() error {
	topics, err := s.repo.ListTopics(1, 500, "")
	if err != nil {
		return err
	}
	for _, t := range topics {
		if err := s.RecalculateTopicHotness(t.ID); err != nil {
			continue
		}
	}
	return nil
}

// IncrementTopicView increments the topic view count and updates hotness.
func (s *Service) IncrementTopicView(topicID int64) error {
	if err := s.repo.EnsureTopicHotness(topicID); err != nil {
		return err
	}
	if err := s.repo.IncrementTopicViewCount(topicID); err != nil {
		return err
	}
	return s.RecalculateTopicHotness(topicID)
}

// GetHotTopics returns hot topics.
func (s *Service) GetHotTopics(limit int) ([]model.Topic, []model.TopicHotness, error) {
	if limit < 1 || limit > 50 {
		limit = 10
	}
	topics, err := s.repo.GetHotTopics(limit)
	if err != nil {
		return nil, nil, err
	}
	var hotnesses []model.TopicHotness
	for _, t := range topics {
		h, _ := s.repo.GetTopicHotness(t.ID)
		if h != nil {
			hotnesses = append(hotnesses, *h)
		}
	}
	return topics, hotnesses, nil
}

// GetTopicWithHotness returns topic details with hotness information.
func (s *Service) GetTopicWithHotness(id int64) (*model.TopicWithReplies, error) {
	// Increment view count
	_ = s.IncrementTopicView(id)

	result, err := s.repo.GetTopicWithReplies(id)
	if err != nil {
		return nil, err
	}
	h, _ := s.repo.GetTopicHotness(id)
	result.Hotness = h
	return result, nil
}

// --- Tags ---

// AddTagsToTopic adds tags to a topic.
func (s *Service) AddTagsToTopic(topicID int64, tagNames []string) error {
	for _, name := range tagNames {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		tag, err := s.repo.GetOrCreateTag(name)
		if err != nil {
			return err
		}
		if err := s.repo.AddTagToTopic(topicID, tag.ID); err != nil {
			return err
		}
	}
	return nil
}

// SetTopicTags replaces the tags on a topic.
func (s *Service) SetTopicTags(topicID int64, tagNames []string) error {
	if err := s.repo.SetTopicTags(topicID, tagNames); err != nil {
		return err
	}
	return nil
}

// GetTopicTags returns tags for a topic.
func (s *Service) GetTopicTags(topicID int64) ([]model.Tag, error) {
	return s.repo.GetTagsByTopicID(topicID)
}

// ListTags lists all tags.
func (s *Service) ListTags() ([]model.Tag, error) {
	return s.repo.ListTags()
}

// ListTopicsByTag filters topics by tag.
func (s *Service) ListTopicsByTag(tagName string, page, limit int) ([]model.Topic, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 20
	}
	topics, err := s.repo.ListTopicsByTag(tagName, page, limit)
	if err != nil {
		return nil, 0, err
	}
	count, err := s.repo.CountTopicsByTag(tagName)
	if err != nil {
		return nil, 0, err
	}
	return topics, count, nil
}

// GetTopicHotness returns hotness data for a topic.
func (s *Service) GetTopicHotness(topicID int64) (*model.TopicHotness, error) {
	return s.repo.GetTopicHotness(topicID)
}

// GetTopicHotnesses returns hotness data for multiple topics.
func (s *Service) GetTopicHotnesses(topicIDs []int64) (map[int64]model.TopicHotness, error) {
	return s.repo.GetTopicHotnesses(topicIDs)
}

// GetTopicTagByName retrieves a tag by name.
func (s *Service) GetTopicTagByName(name string) (*model.Tag, error) {
	return s.repo.GetTagByName(name)
}

// RemoveTopicTag removes a tag from a topic.
func (s *Service) RemoveTopicTag(topicID, tagID int64) error {
	if err := s.repo.RemoveTagFromTopic(topicID, tagID); err != nil {
		return err
	}
	return nil
}

// --- Agent Memory ---

// UpsertMemory creates or updates a memory entry for an agent.
func (s *Service) UpsertMemory(ownerName string, req *model.UpsertMemoryRequest) (*model.AgentMemory, error) {
	owner, err := s.GetMember(ownerName)
	if err != nil {
		return nil, err
	}

	visibility := req.Visibility
	if visibility == "" {
		visibility = "shared"
	}

	var mem *model.AgentMemory
	if req.ID != nil && *req.ID > 0 {
		// Update existing memory - verify ownership
		existing, err := s.repo.GetMemoryByID(*req.ID)
		if err != nil {
			return nil, err
		}
		if existing.MemberID != owner.ID {
			return nil, ErrUnauthorized
		}
		existing.Title = strings.TrimSpace(req.Title)
		existing.Content = strings.TrimSpace(req.Content)
		existing.Category = strings.TrimSpace(req.Category)
		existing.Visibility = visibility
		existing.Version++
		existing.UpdatedAt = time.Now()
		mem = existing
		if err := s.repo.UpdateMemory(mem); err != nil {
			return nil, err
		}
	} else {
		// Create new memory
		mem = &model.AgentMemory{
			MemberID:   owner.ID,
			Title:      strings.TrimSpace(req.Title),
			Content:    strings.TrimSpace(req.Content),
			Category:   strings.TrimSpace(req.Category),
			Visibility: visibility,
			Version:    1,
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		}
		if err := s.repo.CreateMemory(mem); err != nil {
			return nil, err
		}
	}

	// Set tags
	if len(req.Tags) > 0 {
		if err := s.repo.SetMemoryTags(mem.ID, req.Tags); err != nil {
			return nil, err
		}
	}

	// Reload with tags
	return s.repo.GetMemoryByID(mem.ID)
}

// GetMemory retrieves a specific memory by ID.
func (s *Service) GetMemory(id int64) (*model.AgentMemory, error) {
	return s.repo.GetMemoryByID(id)
}

// GetMyMemories retrieves all memory entries for the authenticated agent.
func (s *Service) GetMyMemories(memberName string, page, limit int) ([]model.AgentMemory, int64, error) {
	member, err := s.GetMember(memberName)
	if err != nil {
		return nil, 0, err
	}
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	return s.repo.GetMemberMemories(member.ID, page, limit)
}

// GetSharedMemories retrieves all shared memories across all agents.
func (s *Service) GetSharedMemories(page, limit int) ([]model.AgentMemory, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	return s.repo.GetSharedMemories(page, limit)
}

// GetMemberSharedMemories retrieves shared memories for a specific member.
func (s *Service) GetMemberSharedMemories(memberName string, page, limit int) ([]model.AgentMemory, int64, error) {
	member, err := s.GetMember(memberName)
	if err != nil {
		return nil, 0, err
	}
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	return s.repo.GetSharedMemoriesByMember(member.ID, page, limit)
}

// DeleteMemory deletes a memory entry. Only the owner can delete.
func (s *Service) DeleteMemory(id int64, operatorName string) error {
	mem, err := s.repo.GetMemoryByID(id)
	if err != nil {
		return err
	}
	operator, err := s.GetMember(operatorName)
	if err != nil {
		return err
	}
	if mem.MemberID != operator.ID {
		return ErrUnauthorized
	}
	return s.repo.DeleteMemory(id)
}

// SearchMemories searches shared memories by query.
func (s *Service) SearchMemories(query string, page, limit int) ([]model.AgentMemory, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	return s.repo.SearchMemories(query, page, limit)
}
