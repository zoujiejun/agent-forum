package repository

import (
	"errors"
	"strings"
	"time"

	"github.com/zoujiejun/agent-forum/internal/model"
	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// CreateMember creates a member.
func (r *Repository) CreateMember(m *model.Member) error {
	return r.db.Create(m).Error
}

// GetMemberByName retrieves a member by name.
func (r *Repository) GetMemberByName(name string) (*model.Member, error) {
	var m model.Member
	if err := r.db.Where("name = ?", name).First(&m).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

// GetMemberByID retrieves a member by ID.
func (r *Repository) GetMemberByID(id int64) (*model.Member, error) {
	var m model.Member
	if err := r.db.First(&m, id).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

// ListMembers lists members.
func (r *Repository) ListMembers() ([]model.Member, error) {
	var members []model.Member
	err := r.db.Order("id ASC").Find(&members).Error
	return members, err
}

// CreateTopic creates a topic.
func (r *Repository) CreateTopic(t *model.Topic) error {
	return r.db.Create(t).Error
}

// GetTopic retrieves a topic and its creator.
func (r *Repository) GetTopic(id int64) (*model.Topic, error) {
	var t model.Topic
	if err := r.db.Preload("Creator").First(&t, id).Error; err != nil {
		return nil, err
	}
	return &t, nil
}

// ListTopics retrieves topics with pagination.
func (r *Repository) ListTopics(page, limit int, status string) ([]model.Topic, error) {
	var topics []model.Topic
	query := r.db.Preload("Creator").Preload("Tags").Order("created_at DESC, id DESC")
	if status != "" {
		query = query.Where("status = ?", status)
	}
	offset := (page - 1) * limit
	err := query.Offset(offset).Limit(limit).Find(&topics).Error
	return topics, err
}

func (r *Repository) CountTopics(status string) (int64, error) {
	var count int64
	query := r.db.Model(&model.Topic{})
	if status != "" {
		query = query.Where("status = ?", status)
	}
	err := query.Count(&count).Error
	return count, err
}

// CloseTopic closes a topic.
func (r *Repository) CloseTopic(id int64) error {
	return r.db.Model(&model.Topic{}).
		Where("id = ?", id).
		Updates(map[string]any{"status": "closed", "updated_at": gorm.Expr("CURRENT_TIMESTAMP")}).Error
}

// AddTopicMention adds a mention record.
func (r *Repository) AddTopicMention(tm *model.TopicMention) error {
	return r.db.Create(tm).Error
}

// GetTopicMentions retrieves mention records for a topic.
func (r *Repository) GetTopicMentions(topicID int64) ([]model.TopicMention, error) {
	var mentions []model.TopicMention
	err := r.db.Preload("Member").Where("topic_id = ?", topicID).Find(&mentions).Error
	return mentions, err
}

// CreateReply creates a reply.
func (r *Repository) CreateReply(reply *model.Reply) error {
	return r.db.Create(reply).Error
}

// GetRepliesByTopic retrieves replies for a topic.
func (r *Repository) GetRepliesByTopic(topicID int64) ([]model.Reply, error) {
	var reps []model.Reply
	err := r.db.Preload("Author").Preload("ReplyTo").
		Where("topic_id = ?", topicID).
		Order("created_at ASC, id ASC").
		Find(&reps).Error
	return reps, err
}

// CreateNotification creates a notification.
func (r *Repository) CreateNotification(n *model.Notification) error {
	return r.db.Create(n).Error
}

// GetUnreadNotifications returns unread notifications for a member.
func (r *Repository) GetUnreadNotifications(memberID int64) ([]model.Notification, error) {
	var reps []model.Notification
	err := r.db.Preload("Member").Where("member_id = ? AND read = ?", memberID, false).
		Order("created_at DESC, id DESC").Find(&reps).Error
	return reps, err
}

// MarkNotificationsRead marks notifications as read.
func (r *Repository) MarkNotificationsRead(memberID int64, notificationIDs []int64) error {
	if len(notificationIDs) == 0 {
		return nil
	}
	return r.db.Model(&model.Notification{}).
		Where("id IN ? AND member_id = ?", notificationIDs, memberID).
		Update("read", true).Error
}

// GetMentionedTopics returns topics that still have unread mention notifications.
func (r *Repository) GetMentionedTopics(memberID int64) ([]model.Topic, error) {
	var topics []model.Topic
	subQuery := r.db.Model(&model.Notification{}).
		Select("DISTINCT topic_id").
		Where("member_id = ? AND read = ? AND type = ?", memberID, false, "mention")

	err := r.db.Preload("Creator").
		Where("id IN (?)", subQuery).
		Order("created_at DESC, id DESC").
		Find(&topics).Error
	return topics, err
}

// GetTopicWithReplies retrieves a topic and all of its replies.
func (r *Repository) GetTopicWithReplies(topicID int64) (*model.TopicWithReplies, error) {
	topic, err := r.GetTopic(topicID)
	if err != nil {
		return nil, err
	}

	reps, err := r.GetRepliesByTopic(topicID)
	if err != nil {
		return nil, err
	}

	return &model.TopicWithReplies{
		Topic:   *topic,
		Replies: reps,
	}, nil
}

// GetNotificationsByTopic retrieves a member's notifications for a specific topic.
func (r *Repository) GetNotificationsByTopic(memberID, topicID int64) ([]model.Notification, error) {
	var reps []model.Notification
	err := r.db.Where("member_id = ? AND topic_id = ?", memberID, topicID).
		Order("created_at DESC, id DESC").Find(&reps).Error
	return reps, err
}

// GetTopicByID retrieves basic topic information.
func (r *Repository) GetTopicByID(id int64) (*model.Topic, error) {
	var t model.Topic
	if err := r.db.First(&t, id).Error; err != nil {
		return nil, err
	}
	return &t, nil
}

// CountUnreadMentionNotifications counts unread mention notifications.
func (r *Repository) CountUnreadMentionNotifications(memberID int64) (int64, error) {
	var count int64
	err := r.db.Model(&model.Notification{}).
		Where("member_id = ? AND read = ? AND type = ?", memberID, false, "mention").
		Count(&count).Error
	return count, err
}

// UpdateMemberWorkspace updates a member workspace.
func (r *Repository) UpdateMemberWorkspace(id int64, workspace string) error {
	return r.db.Model(&model.Member{}).Where("id = ?", id).Update("workspace", workspace).Error
}

// --- Tag methods ---

// GetOrCreateTag retrieves or creates a tag.
func (r *Repository) GetOrCreateTag(name string) (*model.Tag, error) {
	name = normalizeTag(name)
	var tag model.Tag
	err := r.db.Where("name = ?", name).First(&tag).Error
	if err == nil {
		return &tag, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	tag = model.Tag{Name: name, CreatedAt: time.Now()}
	if err := r.db.Create(&tag).Error; err != nil {
		// Concurrent creation may have caused a unique key conflict; query again.
		err2 := r.db.Where("name = ?", name).First(&tag).Error
		if err2 != nil {
			return nil, err
		}
		return &tag, nil
	}
	return &tag, nil
}

// GetTagByName retrieves a tag by name.
func (r *Repository) GetTagByName(name string) (*model.Tag, error) {
	var tag model.Tag
	if err := r.db.Where("name = ?", normalizeTag(name)).First(&tag).Error; err != nil {
		return nil, err
	}
	return &tag, nil
}

// ListTags lists all tags.
func (r *Repository) ListTags() ([]model.Tag, error) {
	var tags []model.Tag
	err := r.db.Order("name ASC").Find(&tags).Error
	return tags, err
}

// GetTagsByTopicID retrieves all tags for a topic.
func (r *Repository) GetTagsByTopicID(topicID int64) ([]model.Tag, error) {
	var tags []model.Tag
	err := r.db.
		Joins("JOIN topic_tags ON topic_tags.tag_id = tags.id").
		Where("topic_tags.topic_id = ?", topicID).
		Order("tags.name ASC").
		Find(&tags).Error
	return tags, err
}

// AddTagToTopic adds a tag to a topic.
func (r *Repository) AddTagToTopic(topicID, tagID int64) error {
	tt := model.TopicTag{TopicID: topicID, TagID: tagID, CreatedAt: time.Now()}
	return r.db.Where(model.TopicTag{TopicID: topicID, TagID: tagID}).
		FirstOrCreate(&tt).Error
}

// RemoveTagFromTopic removes a tag from a topic.
func (r *Repository) RemoveTagFromTopic(topicID, tagID int64) error {
	return r.db.Where("topic_id = ? AND tag_id = ?", topicID, tagID).Delete(&model.TopicTag{}).Error
}

// SetTopicTags replaces the tags on a topic.
func (r *Repository) SetTopicTags(topicID int64, tagNames []string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("topic_id = ?", topicID).Delete(&model.TopicTag{}).Error; err != nil {
			return err
		}
		for _, name := range tagNames {
			name = normalizeTag(name)
			if name == "" {
				continue
			}
			var tag model.Tag
			if err := tx.Where("name = ?", name).First(&tag).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					tag = model.Tag{Name: name, CreatedAt: time.Now()}
					if err := tx.Create(&tag).Error; err != nil {
						return err
					}
				} else {
					return err
				}
			}
			tt := model.TopicTag{TopicID: topicID, TagID: tag.ID, CreatedAt: time.Now()}
			if err := tx.Where(model.TopicTag{TopicID: topicID, TagID: tag.ID}).FirstOrCreate(&tt).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// ListTopicsByTag retrieves topics with a specific tag.
func (r *Repository) ListTopicsByTag(tagName string, page, limit int) ([]model.Topic, error) {
	var topics []model.Topic
	offset := (page - 1) * limit
	err := r.db.
		Joins("JOIN topic_tags ON topic_tags.topic_id = topics.id").
		Joins("JOIN tags ON tags.id = topic_tags.tag_id").
		Preload("Creator").
		Preload("Tags").
		Where("tags.name = ?", normalizeTag(tagName)).
		Order("topics.created_at DESC, topics.id DESC").
		Offset(offset).Limit(limit).
		Find(&topics).Error
	return topics, err
}

// CountTopicsByTag counts topics with a specific tag.
func (r *Repository) CountTopicsByTag(tagName string) (int64, error) {
	var count int64
	err := r.db.Model(&model.Topic{}).
		Joins("JOIN topic_tags ON topic_tags.topic_id = topics.id").
		Joins("JOIN tags ON tags.id = topic_tags.tag_id").
		Where("tags.name = ?", normalizeTag(tagName)).
		Count(&count).Error
	return count, err
}

// GetHotTopics retrieves hot topics sorted by score.
func (r *Repository) GetHotTopics(limit int) ([]model.Topic, error) {
	var topics []model.Topic
	err := r.db.
		Joins("LEFT JOIN topic_hotnesses ON topic_hotnesses.topic_id = topics.id").
		Preload("Creator").
		Preload("Tags").
		Order("COALESCE(topic_hotnesses.hotness, 0) DESC, topics.created_at DESC").
		Limit(limit).
		Find(&topics).Error
	return topics, err
}

// GetTopicHotness retrieves topic hotness data.
func (r *Repository) GetTopicHotness(topicID int64) (*model.TopicHotness, error) {
	var h model.TopicHotness
	if err := r.db.Where("topic_id = ?", topicID).First(&h).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &h, nil
}

// UpsertTopicHotness creates or updates topic hotness data.
func (r *Repository) UpsertTopicHotness(h *model.TopicHotness) error {
	h.UpdatedAt = time.Now()
	return r.db.Where("topic_id = ?", h.TopicID).
		Assign(model.TopicHotness{ViewCount: h.ViewCount, LikeCount: h.LikeCount, Hotness: h.Hotness, UpdatedAt: h.UpdatedAt}).
		FirstOrCreate(h).Error
}

// IncrementTopicViewCount increments the topic view count.
func (r *Repository) IncrementTopicViewCount(topicID int64) error {
	return r.db.Model(&model.TopicHotness{}).
		Where("topic_id = ?", topicID).
		UpdateColumn("view_count", gorm.Expr("view_count + 1")).Error
}

// EnsureTopicHotness ensures a topic hotness record exists with zero values.
func (r *Repository) EnsureTopicHotness(topicID int64) error {
	th := &model.TopicHotness{TopicID: topicID, ViewCount: 0, LikeCount: 0, Hotness: 0, UpdatedAt: time.Now()}
	return r.db.Where(model.TopicHotness{TopicID: topicID}).FirstOrCreate(th).Error
}

// GetTopicHotnesses retrieves hotness data for multiple topics.
func (r *Repository) GetTopicHotnesses(topicIDs []int64) (map[int64]model.TopicHotness, error) {
	var rows []model.TopicHotness
	if len(topicIDs) == 0 {
		return nil, nil
	}
	err := r.db.Where("topic_id IN ?", topicIDs).Find(&rows).Error
	if err != nil {
		return nil, err
	}
	result := make(map[int64]model.TopicHotness, len(rows))
	for _, row := range rows {
		result[row.TopicID] = row
	}
	return result, nil
}

// normalizeTag normalizes a tag name (lowercase, trimmed).
func normalizeTag(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ToLower(name)
	return name
}

// --- Agent Memory Repository Methods ---

// CreateMemory creates a new agent memory entry.
func (r *Repository) CreateMemory(m *model.AgentMemory) error {
	return r.db.Create(m).Error
}

// GetMemoryByID retrieves a memory entry by its ID.
func (r *Repository) GetMemoryByID(id int64) (*model.AgentMemory, error) {
	var mem model.AgentMemory
	if err := r.db.Preload("Member").First(&mem, id).Error; err != nil {
		return nil, err
	}
	return &mem, nil
}

// GetMemoryByMemberID retrieves all memory entries for a member.
func (r *Repository) GetMemoryByMemberID(memberID int64) ([]model.AgentMemory, error) {
	var mems []model.AgentMemory
	err := r.db.Preload("Tags").
		Where("member_id = ?", memberID).
		Order("updated_at DESC, id DESC").
		Find(&mems).Error
	return mems, err
}

// GetSharedMemories retrieves all shared memory entries across all agents.
func (r *Repository) GetSharedMemories(page, limit int) ([]model.AgentMemory, int64, error) {
	var mems []model.AgentMemory
	var total int64

	if err := r.db.Model(&model.AgentMemory{}).Where("visibility = ?", "shared").Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	err := r.db.Preload("Member").Preload("Tags").
		Where("visibility = ?", "shared").
		Order("updated_at DESC, id DESC").
		Offset(offset).Limit(limit).
		Find(&mems).Error
	return mems, total, err
}

// GetSharedMemoriesByMember retrieves shared memory entries for a specific member.
func (r *Repository) GetSharedMemoriesByMember(memberID int64, page, limit int) ([]model.AgentMemory, int64, error) {
	var mems []model.AgentMemory
	var total int64

	if err := r.db.Model(&model.AgentMemory{}).
		Where("member_id = ? AND visibility = ?", memberID, "shared").
		Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	err := r.db.Preload("Member").Preload("Tags").
		Where("member_id = ? AND visibility = ?", memberID, "shared").
		Order("updated_at DESC, id DESC").
		Offset(offset).Limit(limit).
		Find(&mems).Error
	return mems, total, err
}

// GetMemberMemories retrieves all memory entries for a member (both shared and private).
func (r *Repository) GetMemberMemories(memberID int64, page, limit int) ([]model.AgentMemory, int64, error) {
	var mems []model.AgentMemory
	var total int64

	if err := r.db.Model(&model.AgentMemory{}).
		Where("member_id = ?", memberID).
		Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	err := r.db.Preload("Tags").
		Where("member_id = ?", memberID).
		Order("updated_at DESC, id DESC").
		Offset(offset).Limit(limit).
		Find(&mems).Error
	return mems, total, err
}

// UpdateMemory updates an existing memory entry.
func (r *Repository) UpdateMemory(m *model.AgentMemory) error {
	return r.db.Save(m).Error
}

// DeleteMemory deletes a memory entry by ID.
func (r *Repository) DeleteMemory(id int64) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		// Delete memory tag associations first
		if err := tx.Where("memory_id = ?", id).Delete(&model.MemoryTag{}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.AgentMemory{}, id).Error
	})
}

// SetMemoryTags sets tags for a memory entry (replace mode).
func (r *Repository) SetMemoryTags(memoryID int64, tagNames []string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		// Delete existing tags
		if err := tx.Where("memory_id = ?", memoryID).Delete(&model.MemoryTag{}).Error; err != nil {
			return err
		}
		// Add new tags
		for _, name := range tagNames {
			name = normalizeTag(name)
			if name == "" {
				continue
			}
			var tag model.Tag
			if err := tx.Where("name = ?", name).First(&tag).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					tag = model.Tag{Name: name, CreatedAt: time.Now()}
					if err := tx.Create(&tag).Error; err != nil {
						return err
					}
				} else {
					return err
				}
			}
			mt := model.MemoryTag{MemoryID: memoryID, TagID: tag.ID, CreatedAt: time.Now()}
			if err := tx.Where(model.MemoryTag{MemoryID: memoryID, TagID: tag.ID}).FirstOrCreate(&mt).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// SearchMemories searches memories by query string in title or content.
func (r *Repository) SearchMemories(query string, page, limit int) ([]model.AgentMemory, int64, error) {
	var mems []model.AgentMemory
	var total int64
	q := "%" + query + "%"

	if err := r.db.Model(&model.AgentMemory{}).
		Where("visibility = ? AND (title LIKE ? OR content LIKE ?)", "shared", q, q).
		Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	err := r.db.Preload("Member").Preload("Tags").
		Where("visibility = ? AND (title LIKE ? OR content LIKE ?)", "shared", q, q).
		Order("updated_at DESC, id DESC").
		Offset(offset).Limit(limit).
		Find(&mems).Error
	return mems, total, err
}
