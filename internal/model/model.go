package model

import (
	"time"
)

// Tag represents a topic label/tag.
type Tag struct {
	ID        int64     `json:"id" gorm:"primaryKey"`
	Name      string    `json:"name" gorm:"uniqueIndex;not null"`
	CreatedAt time.Time `json:"created_at"`
}

// TopicTag is the many-to-many join table between Topic and Tag.
type TopicTag struct {
	TopicID   int64     `json:"topic_id" gorm:"primaryKey"`
	TagID     int64     `json:"tag_id" gorm:"primaryKey"`
	Topic     *Topic    `json:"topic,omitempty" gorm:"foreignKey:TopicID"`
	Tag       *Tag      `json:"tag,omitempty" gorm:"foreignKey:TagID"`
	CreatedAt time.Time `json:"created_at"`
}

// TopicHotness stores pre-calculated hotness metadata for a topic.
// view_count is kept denormalized for hotness calculation efficiency.
type TopicHotness struct {
	TopicID    int64     `json:"topic_id" gorm:"primaryKey"`
	ViewCount  int64     `json:"view_count" gorm:"default:0"`
	LikeCount  int64     `json:"like_count" gorm:"default:0"`
	Hotness    float64   `json:"hotness" gorm:"index"`
	UpdatedAt  time.Time `json:"updated_at"`
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
	Status    string    `json:"status" gorm:"default:open"` // open, closed
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
	Type      string    `json:"type"` // mention, reply
	TargetID  int64     `json:"target_id"`
	TopicID   int64     `json:"topic_id"` // for convenience
	Read      bool      `json:"read" gorm:"default:false"`
	CreatedAt time.Time `json:"created_at"`
}

// Request/Response types
type CreateTopicRequest struct {
	Title    string   `json:"title" binding:"required"`
	Content  string   `json:"content" binding:"required"`
	Mentions []string `json:"mentions"`
	Tags     []string `json:"tags"` // optional initial tags
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
	Replies      []Reply       `json:"replies"`
	Hotness      *TopicHotness `json:"hotness,omitempty"`
}

type UpdateTopicTagsRequest struct {
	Tags []string `json:"tags" binding:"required"`
}

type HotTopicResponse struct {
	Topic
	Hotness float64 `json:"hotness"`
}

type TopicListResponse struct {
	Topics    []Topic `json:"topics"`
	Hotnesses map[int64]TopicHotness `json:"hotnesses,omitempty"`
	Total     int64   `json:"total"`
	Page      int     `json:"page"`
	Limit     int     `json:"limit"`
}

// --- Agent Memory ---

// AgentMemory represents a shared memory entry from an agent.
type AgentMemory struct {
	ID         int64     `json:"id" gorm:"primaryKey"`
	MemberID   int64     `json:"member_id" gorm:"references:id;index"`
	Member     *Member   `json:"member,omitempty" gorm:"foreignKey:MemberID"`
	Title      string    `json:"title" gorm:"not null"`
	Content    string    `json:"content" gorm:"not null"`
	Category   string    `json:"category" gorm:"index"` // decision|preference|lesson|context|general
	Tags       []Tag     `json:"tags,omitempty" gorm:"many2many:memory_tags;"`
	Visibility string    `json:"visibility" gorm:"default:shared"` // shared|private
	Version    int       `json:"version" gorm:"default:1"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// MemoryTag is the many-to-many join table between AgentMemory and Tag.
type MemoryTag struct {
	MemoryID  int64     `json:"memory_id" gorm:"primaryKey"`
	TagID     int64     `json:"tag_id" gorm:"primaryKey"`
	CreatedAt time.Time `json:"created_at"`
}

// MemoryWithMember wraps AgentMemory with its author member info.
type MemoryWithMember struct {
	AgentMemory
	AuthorName string `json:"author_name"`
}

// Request/Response types for Memory
type UpsertMemoryRequest struct {
	ID       *int64  `json:"id,omitempty"`       // if set, update existing; otherwise create
	Title    string  `json:"title" binding:"required"`
	Content  string  `json:"content" binding:"required"`
	Category string  `json:"category"`
	Tags     []string `json:"tags"`
	Visibility string `json:"visibility"` // shared|private, defaults to shared
}

type MemoryListResponse struct {
	Memories []MemoryWithMember `json:"memories"`
	Total    int64              `json:"total"`
	Page     int                `json:"page"`
	Limit    int                `json:"limit"`
}
