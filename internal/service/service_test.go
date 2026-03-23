package service

import (
	"testing"

	"github.com/zoujiejun/agent-forum/internal/model"
	"github.com/zoujiejun/agent-forum/internal/repository"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newTestService(t *testing.T) *Service {
	t.Helper()
	dsn := "file:service_test_" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Member{}, &model.Topic{}, &model.TopicMention{}, &model.Reply{}, &model.Notification{}, &model.Tag{}, &model.TopicTag{}, &model.TopicHotness{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return New(repository.New(db))
}

func registerMembers(t *testing.T, svc *Service, names ...string) {
	t.Helper()
	for _, name := range names {
		if _, err := svc.RegisterMember(name, "workspace-test"); err != nil {
			t.Fatalf("register %s: %v", name, err)
		}
	}
}

func TestCreateTopicDeduplicatesMentionsAndSkipsSelf(t *testing.T) {
	svc := newTestService(t)
	registerMembers(t, svc, "agent-alpha", "agent-beta")

	topic, err := svc.CreateTopic("agent-alpha", "Test Title", "Test Content", []string{"@agent-beta", "agent-beta", "@agent-alpha", ""}, nil)
	if err != nil {
		t.Fatalf("CreateTopic error: %v", err)
	}

	notifications, err := svc.GetNotifications("agent-beta")
	if err != nil {
		t.Fatalf("GetNotifications error: %v", err)
	}
	if len(notifications) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(notifications))
	}
	if notifications[0].Type != "mention" || notifications[0].TopicID != topic.ID {
		t.Fatalf("unexpected notification: %+v", notifications[0])
	}

	creatorNotifications, err := svc.GetNotifications("agent-alpha")
	if err != nil {
		t.Fatalf("GetNotifications creator error: %v", err)
	}
	if len(creatorNotifications) != 0 {
		t.Fatalf("expected creator to have no notifications, got %d", len(creatorNotifications))
	}
}

func TestCloseTopicRequiresCreator(t *testing.T) {
	svc := newTestService(t)
	registerMembers(t, svc, "agent-alpha", "agent-beta")

	topic, err := svc.CreateTopic("agent-alpha", "Test Title", "Test Content", nil, nil)
	if err != nil {
		t.Fatalf("CreateTopic error: %v", err)
	}

	if err := svc.CloseTopic(topic.ID, "agent-beta"); err != ErrUnauthorized {
		t.Fatalf("expected ErrUnauthorized, got %v", err)
	}

	if err := svc.CloseTopic(topic.ID, "agent-alpha"); err != nil {
		t.Fatalf("creator close topic error: %v", err)
	}
}

func TestGetMentionedTopicsOnlyReturnsUnreadMentions(t *testing.T) {
	svc := newTestService(t)
	registerMembers(t, svc, "agent-alpha", "agent-beta")

	topic, err := svc.CreateTopic("agent-alpha", "Test Title", "Test Content", []string{"@agent-beta"}, nil)
	if err != nil {
		t.Fatalf("CreateTopic error: %v", err)
	}

	topics, err := svc.GetMentionedTopics("agent-beta")
	if err != nil {
		t.Fatalf("GetMentionedTopics error: %v", err)
	}
	if len(topics) != 1 || topics[0].ID != topic.ID {
		t.Fatalf("unexpected mentioned topics: %+v", topics)
	}

	notifications, err := svc.GetNotifications("agent-beta")
	if err != nil {
		t.Fatalf("GetNotifications error: %v", err)
	}
	ids := []int64{notifications[0].ID}
	if err := svc.MarkNotificationsRead("agent-beta", ids); err != nil {
		t.Fatalf("MarkNotificationsRead error: %v", err)
	}

	topics, err = svc.GetMentionedTopics("agent-beta")
	if err != nil {
		t.Fatalf("GetMentionedTopics after read error: %v", err)
	}
	if len(topics) != 0 {
		t.Fatalf("expected no unread mentioned topics, got %+v", topics)
	}
}

func TestCreateReplyNotifiesParticipantsWithoutDuplicates(t *testing.T) {
	svc := newTestService(t)
	registerMembers(t, svc, "agent-alpha", "agent-beta", "agent-gamma")

	topic, err := svc.CreateTopic("agent-alpha", "Test Title", "Test Content", []string{"@agent-beta", "@agent-gamma"}, nil)
	if err != nil {
		t.Fatalf("CreateTopic error: %v", err)
	}

	if _, err := svc.CreateReply(topic.ID, "agent-beta", "Acknowledged", nil); err != nil {
		t.Fatalf("CreateReply error: %v", err)
	}

	creatorNotifications, err := svc.GetNotifications("agent-alpha")
	if err != nil {
		t.Fatalf("GetNotifications creator error: %v", err)
	}
	if len(creatorNotifications) != 1 || creatorNotifications[0].Type != "reply" {
		t.Fatalf("unexpected creator notifications: %+v", creatorNotifications)
	}

	mentionedNotifications, err := svc.GetNotifications("agent-gamma")
	if err != nil {
		t.Fatalf("GetNotifications mentioned error: %v", err)
	}
	var replyCount int
	for _, n := range mentionedNotifications {
		if n.Type == "reply" {
			replyCount++
		}
	}
	if replyCount != 1 {
		t.Fatalf("expected 1 reply notification for agent-gamma, got %d (%+v)", replyCount, mentionedNotifications)
	}
}

func TestCreateReplyCreatesMentionNotificationsFromReplyContent(t *testing.T) {
	svc := newTestService(t)
	registerMembers(t, svc, "agent-alpha", "agent-beta", "agent-gamma", "agent-delta")

	topic, err := svc.CreateTopic("agent-alpha", "Test Title", "Test Content", []string{"@agent-beta"}, nil)
	if err != nil {
		t.Fatalf("CreateTopic error: %v", err)
	}

	if _, err := svc.CreateReply(topic.ID, "agent-beta", "@agent-gamma @agent-delta Please follow up, @agent-gamma", nil); err != nil {
		t.Fatalf("CreateReply error: %v", err)
	}

	qianxueTopics, err := svc.GetMentionedTopics("agent-gamma")
	if err != nil {
		t.Fatalf("GetMentionedTopics agent-gamma error: %v", err)
	}
	if len(qianxueTopics) != 1 || qianxueTopics[0].ID != topic.ID {
		t.Fatalf("unexpected agent-gamma mentioned topics: %+v", qianxueTopics)
	}

	wanxingTopics, err := svc.GetMentionedTopics("agent-delta")
	if err != nil {
		t.Fatalf("GetMentionedTopics agent-delta error: %v", err)
	}
	if len(wanxingTopics) != 1 || wanxingTopics[0].ID != topic.ID {
		t.Fatalf("unexpected agent-delta mentioned topics: %+v", wanxingTopics)
	}

	qianxueNotifications, err := svc.GetNotifications("agent-gamma")
	if err != nil {
		t.Fatalf("GetNotifications agent-gamma error: %v", err)
	}
	var qianxueMentionCount int
	for _, n := range qianxueNotifications {
		if n.Type == "mention" {
			qianxueMentionCount++
		}
	}
	if qianxueMentionCount != 1 {
		t.Fatalf("expected 1 mention notification for agent-gamma, got %d (%+v)", qianxueMentionCount, qianxueNotifications)
	}
}

func TestExtractMentionNamesSupportsMarkdownWrappedMentions(t *testing.T) {
	names := extractMentionNames("**@agent-delta** and __@agent-gamma__ should be parsed, along with plain @agent-beta and `@agent-alpha`.")
	if len(names) != 4 {
		t.Fatalf("expected 4 names, got %d (%+v)", len(names), names)
	}
	if names[0] != "agent-delta" || names[1] != "agent-gamma" || names[2] != "agent-beta" || names[3] != "agent-alpha" {
		t.Fatalf("unexpected names: %+v", names)
	}
}

func TestMergeMentionNamesDeduplicatesAcrossSources(t *testing.T) {
	names := mergeMentionNames([]string{"@agent-b", "agent-c"}, []string{"agent-c", "agent-d", "@agent-b"})
	if len(names) != 3 {
		t.Fatalf("expected 3 names, got %d (%+v)", len(names), names)
	}
	if names[0] != "agent-b" || names[1] != "agent-c" || names[2] != "agent-d" {
		t.Fatalf("unexpected names: %+v", names)
	}
}
