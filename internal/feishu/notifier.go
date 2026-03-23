package feishu

import (
	"fmt"
	"strings"

	"github.com/zoujiejun/agent-forum/internal/model"
)

// TopicCreatedNotifyParams contains data for a topic-created notification.
type TopicCreatedNotifyParams struct {
	Topic   *model.Topic
	Creator *model.Member
}

// ReplyCreatedNotifyParams contains data for a reply-created notification.
type ReplyCreatedNotifyParams struct {
	Topic  *model.Topic
	Reply  *model.Reply
	Author *model.Member
}

// Notifier sends Feishu notifications.
type Notifier struct {
	client  *Client
	chatID  string
	enabled bool
}

// NewNotifier creates a Feishu notifier. Pass empty strings to disable.
func NewNotifier(appID, appSecret, chatID string) *Notifier {
	if appID == "" || appSecret == "" || chatID == "" {
		return &Notifier{enabled: false}
	}
	return &Notifier{
		client:  NewClient(appID, appSecret),
		chatID:  chatID,
		enabled: true,
	}
}

// IsEnabled returns whether the notifier is configured.
func (n *Notifier) IsEnabled() bool {
	return n.enabled
}

// NotifyTopicCreated sends a notification when a new topic is created.
func (n *Notifier) NotifyTopicCreated(params TopicCreatedNotifyParams) error {
	if !n.enabled {
		return nil
	}

	var tagStr string
	if len(params.Topic.Tags) > 0 {
		tags := make([]string, len(params.Topic.Tags))
		for i, t := range params.Topic.Tags {
			tags[i] = t.Name
		}
		tagStr = fmt.Sprintf(" | 🏷️ %s", strings.Join(tags, ", "))
	}

	content := fmt.Sprintf(
		"🆕 **%s**\n\n👤 %s created a new topic in Agent Forum%s\n\n📝 %s",
		escapeMarkdown(params.Topic.Title),
		escapeMarkdown(params.Creator.Name),
		tagStr,
		truncate(params.Topic.Content, 200),
	)

	title := fmt.Sprintf("New topic: %s", params.Topic.Title)
	return n.client.SendRichMessage(n.chatID, title, content)
}

// NotifyReplyCreated sends a notification when a reply is created.
func (n *Notifier) NotifyReplyCreated(params ReplyCreatedNotifyParams) error {
	if !n.enabled {
		return nil
	}

	content := fmt.Sprintf(
		"💬 %s replied in **%s**:\n\n%s",
		escapeMarkdown(params.Author.Name),
		escapeMarkdown(params.Topic.Title),
		truncate(params.Reply.Content, 300),
	)

	title := fmt.Sprintf("New reply: %s", params.Topic.Title)
	return n.client.SendRichMessage(n.chatID, title, content)
}

// NotifyMention sends a notification when a member is mentioned.
func (n *Notifier) NotifyMention(topic *model.Topic, mentionedBy, mentionedMember string) error {
	if !n.enabled {
		return nil
	}

	content := fmt.Sprintf(
		"👋 %s mentioned you in **%s**",
		escapeMarkdown(mentionedBy),
		escapeMarkdown(topic.Title),
	)

	return n.client.SendTextMessage(n.chatID, content)
}

func escapeMarkdown(s string) string {
	// Escape special markdown characters for Feishu markdown
	s = strings.ReplaceAll(s, "*", "\\*")
	s = strings.ReplaceAll(s, "_", "\\_")
	s = strings.ReplaceAll(s, "`", "\\`")
	s = strings.ReplaceAll(s, "[", "\\[")
	s = strings.ReplaceAll(s, "]", "\\]")
	return s
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	runes := []rune(s)
	return string(runes[:maxLen]) + "…"
}
