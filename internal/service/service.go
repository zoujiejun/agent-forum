package service

import (
	"errors"
	"regexp"
	"strings"
	"time"

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
	repo *repository.Repository
}

func New(repo *repository.Repository) *Service {
	return &Service{repo: repo}
}

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

func (s *Service) ListMembers() ([]model.Member, error) {
	return s.repo.ListMembers()
}

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
	}

	return s.repo.GetTopic(topic.ID)
}

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

	return s.repo.CloseTopic(id)
}

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

	return s.repo.GetRepliesByTopic(topicID)
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
		return s.repo.CreateNotification(notif)
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

func (s *Service) GetNotifications(memberName string) ([]model.Notification, error) {
	member, err := s.GetMember(memberName)
	if err != nil {
		return nil, err
	}
	return s.repo.GetUnreadNotifications(member.ID)
}

func (s *Service) MarkNotificationsRead(memberName string, ids []int64) error {
	member, err := s.GetMember(memberName)
	if err != nil {
		return err
	}
	return s.repo.MarkNotificationsRead(member.ID, ids)
}

func (s *Service) GetMentionedTopics(memberName string) ([]model.Topic, error) {
	member, err := s.GetMember(memberName)
	if err != nil {
		return nil, err
	}
	return s.repo.GetMentionedTopics(member.ID)
}

func (s *Service) GetUnreadMentionCount(memberName string) (int64, error) {
	member, err := s.GetMember(memberName)
	if err != nil {
		return 0, err
	}
	return s.repo.CountUnreadMentionNotifications(member.ID)
}

func (s *Service) UpdateMemberWorkspace(id int64, workspace string) error {
	return s.repo.UpdateMemberWorkspace(id, workspace)
}

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

func (s *Service) SetTopicTags(topicID int64, tagNames []string) error {
	return s.repo.SetTopicTags(topicID, tagNames)
}

func (s *Service) GetTopicTags(topicID int64) ([]model.Tag, error) {
	return s.repo.GetTagsByTopicID(topicID)
}

func (s *Service) ListTags() ([]model.Tag, error) {
	return s.repo.ListTags()
}

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

func (s *Service) GetTopicTagByName(name string) (*model.Tag, error) {
	return s.repo.GetTagByName(name)
}

func (s *Service) RemoveTopicTag(topicID, tagID int64) error {
	return s.repo.RemoveTagFromTopic(topicID, tagID)
}
