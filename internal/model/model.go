package model

import (
	"time"
)

type Tag struct {
	ID        int64     `json:"id" gorm:"primaryKey"`
	Name      string    `json:"name" gorm:"uniqueIndex;not null"`
	CreatedAt time.Time `json:"created_at"`
}

type TopicTag struct {
	TopicID   int64     `json:"topic_id" gorm:"primaryKey"`
	TagID     int64     `json:"tag_id" gorm:"primaryKey"`
	Topic     *Topic    `json:"topic,omitempty" gorm:"foreignKey:TopicID"`
	Tag       *Tag      `json:"tag,omitempty" gorm:"foreignKey:TagID"`
	CreatedAt time.Time `json:"created_at"`
}

type Member struct {
	ID        int64     `json:"id" gorm:"primaryKey"`
	Name      string    `json:"name" gorm:"uniqueIndex;not null"`
	Workspace string    `json:"workspace" gorm:"not null"`
	CreatedAt time.Time `json:"created_at"`
}

type Topic struct {
	ID        int64     `json:"id" gorm:"primaryKey"`
	Title     string    `json:"title" gorm:"not null"`
	Content   string    `json:"content" gorm:"not null"`
	CreatorID int64     `json:"creator_id" gorm:"references:id"`
	Creator   *Member   `json:"creator,omitempty" gorm:"foreignKey:CreatorID"`
	Status    string    `json:"status" gorm:"default:open"`
	Tags      []Tag     `json:"tags,omitempty" gorm:"many2many:topic_tags;"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type TopicMention struct {
	ID          int64     `json:"id" gorm:"primaryKey"`
	TopicID     int64     `json:"topic_id" gorm:"references:id"`
	MemberID    int64     `json:"member_id" gorm:"references:id"`
	Member      *Member   `json:"member,omitempty" gorm:"foreignKey:MemberID"`
	MentionedAt time.Time `json:"mentioned_at"`
}

type Reply struct {
	ID        int64     `json:"id" gorm:"primaryKey"`
	TopicID   int64     `json:"topic_id" gorm:"references:id;index"`
	Topic     *Topic    `json:"topic,omitempty" gorm:"foreignKey:TopicID"`
	AuthorID  int64     `json:"author_id" gorm:"references:id"`
	Author    *Member   `json:"author,omitempty" gorm:"foreignKey:AuthorID"`
	Content   string    `json:"content" gorm:"not null"`
	ReplyToID *int64    `json:"reply_to_id"`
	ReplyTo   *Reply    `json:"reply_to,omitempty" gorm:"foreignKey:ReplyToID"`
	CreatedAt time.Time `json:"created_at"`
}

type Notification struct {
	ID        int64     `json:"id" gorm:"primaryKey"`
	MemberID  int64     `json:"member_id" gorm:"references:id;index"`
	Member    *Member   `json:"member,omitempty" gorm:"foreignKey:MemberID"`
	Type      string    `json:"type"`
	TargetID  int64     `json:"target_id"`
	TopicID   int64     `json:"topic_id"`
	Read      bool      `json:"read" gorm:"default:false"`
	CreatedAt time.Time `json:"created_at"`
}

type CreateTopicRequest struct {
	Title    string   `json:"title" binding:"required"`
	Content  string   `json:"content" binding:"required"`
	Mentions []string `json:"mentions"`
	Tags     []string `json:"tags"`
}

type CreateReplyRequest struct {
	Content   string `json:"content" binding:"required"`
	ReplyToID *int64 `json:"reply_to_id"`
}

type RegisterMemberRequest struct {
	Name      string `json:"name" binding:"required"`
	Workspace string `json:"workspace" binding:"required"`
}

type TopicWithReplies struct {
	Topic
	Replies []Reply `json:"replies"`
}

type UpdateTopicTagsRequest struct {
	Tags []string `json:"tags" binding:"required"`
}

type TopicListResponse struct {
	Topics []Topic `json:"topics"`
	Total  int64   `json:"total"`
	Page   int     `json:"page"`
	Limit  int     `json:"limit"`
}
