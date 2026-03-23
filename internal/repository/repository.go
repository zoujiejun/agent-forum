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

func (r *Repository) CreateMember(m *model.Member) error {
	return r.db.Create(m).Error
}

func (r *Repository) GetMemberByName(name string) (*model.Member, error) {
	var m model.Member
	if err := r.db.Where("name = ?", name).First(&m).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *Repository) GetMemberByID(id int64) (*model.Member, error) {
	var m model.Member
	if err := r.db.First(&m, id).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *Repository) ListMembers() ([]model.Member, error) {
	var members []model.Member
	err := r.db.Order("id ASC").Find(&members).Error
	return members, err
}

func (r *Repository) CreateTopic(t *model.Topic) error {
	return r.db.Create(t).Error
}

func (r *Repository) GetTopic(id int64) (*model.Topic, error) {
	var t model.Topic
	if err := r.db.Preload("Creator").Preload("Tags").First(&t, id).Error; err != nil {
		return nil, err
	}
	return &t, nil
}

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

func (r *Repository) CloseTopic(id int64) error {
	return r.db.Model(&model.Topic{}).
		Where("id = ?", id).
		Updates(map[string]any{"status": "closed", "updated_at": gorm.Expr("CURRENT_TIMESTAMP")}).Error
}

func (r *Repository) AddTopicMention(tm *model.TopicMention) error {
	return r.db.Create(tm).Error
}

func (r *Repository) GetTopicMentions(topicID int64) ([]model.TopicMention, error) {
	var mentions []model.TopicMention
	err := r.db.Preload("Member").Where("topic_id = ?", topicID).Find(&mentions).Error
	return mentions, err
}

func (r *Repository) CreateReply(reply *model.Reply) error {
	return r.db.Create(reply).Error
}

func (r *Repository) GetRepliesByTopic(topicID int64) ([]model.Reply, error) {
	var reps []model.Reply
	err := r.db.Preload("Author").Preload("ReplyTo").
		Where("topic_id = ?", topicID).
		Order("created_at ASC, id ASC").
		Find(&reps).Error
	return reps, err
}

func (r *Repository) CreateNotification(n *model.Notification) error {
	return r.db.Create(n).Error
}

func (r *Repository) GetUnreadNotifications(memberID int64) ([]model.Notification, error) {
	var reps []model.Notification
	err := r.db.Preload("Member").Where("member_id = ? AND read = ?", memberID, false).
		Order("created_at DESC, id DESC").Find(&reps).Error
	return reps, err
}

func (r *Repository) MarkNotificationsRead(memberID int64, notificationIDs []int64) error {
	if len(notificationIDs) == 0 {
		return nil
	}
	return r.db.Model(&model.Notification{}).
		Where("id IN ? AND member_id = ?", notificationIDs, memberID).
		Update("read", true).Error
}

func (r *Repository) GetMentionedTopics(memberID int64) ([]model.Topic, error) {
	var topics []model.Topic
	subQuery := r.db.Model(&model.Notification{}).
		Select("DISTINCT topic_id").
		Where("member_id = ? AND read = ? AND type = ?", memberID, false, "mention")

	err := r.db.Preload("Creator").Preload("Tags").
		Where("id IN (?)", subQuery).
		Order("created_at DESC, id DESC").
		Find(&topics).Error
	return topics, err
}

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

func (r *Repository) GetNotificationsByTopic(memberID, topicID int64) ([]model.Notification, error) {
	var reps []model.Notification
	err := r.db.Where("member_id = ? AND topic_id = ?", memberID, topicID).
		Order("created_at DESC, id DESC").Find(&reps).Error
	return reps, err
}

func (r *Repository) GetTopicByID(id int64) (*model.Topic, error) {
	var t model.Topic
	if err := r.db.First(&t, id).Error; err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *Repository) CountUnreadMentionNotifications(memberID int64) (int64, error) {
	var count int64
	err := r.db.Model(&model.Notification{}).
		Where("member_id = ? AND read = ? AND type = ?", memberID, false, "mention").
		Count(&count).Error
	return count, err
}

func (r *Repository) UpdateMemberWorkspace(id int64, workspace string) error {
	return r.db.Model(&model.Member{}).Where("id = ?", id).Update("workspace", workspace).Error
}

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
		err2 := r.db.Where("name = ?", name).First(&tag).Error
		if err2 != nil {
			return nil, err
		}
		return &tag, nil
	}
	return &tag, nil
}

func (r *Repository) GetTagByName(name string) (*model.Tag, error) {
	var tag model.Tag
	if err := r.db.Where("name = ?", normalizeTag(name)).First(&tag).Error; err != nil {
		return nil, err
	}
	return &tag, nil
}

func (r *Repository) ListTags() ([]model.Tag, error) {
	var tags []model.Tag
	err := r.db.Order("name ASC").Find(&tags).Error
	return tags, err
}

func (r *Repository) GetTagsByTopicID(topicID int64) ([]model.Tag, error) {
	var tags []model.Tag
	err := r.db.
		Joins("JOIN topic_tags ON topic_tags.tag_id = tags.id").
		Where("topic_tags.topic_id = ?", topicID).
		Order("tags.name ASC").
		Find(&tags).Error
	return tags, err
}

func (r *Repository) AddTagToTopic(topicID, tagID int64) error {
	tt := model.TopicTag{TopicID: topicID, TagID: tagID, CreatedAt: time.Now()}
	return r.db.Where(model.TopicTag{TopicID: topicID, TagID: tagID}).
		FirstOrCreate(&tt).Error
}

func (r *Repository) RemoveTagFromTopic(topicID, tagID int64) error {
	return r.db.Where("topic_id = ? AND tag_id = ?", topicID, tagID).Delete(&model.TopicTag{}).Error
}

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

func (r *Repository) CountTopicsByTag(tagName string) (int64, error) {
	var count int64
	err := r.db.Model(&model.Topic{}).
		Joins("JOIN topic_tags ON topic_tags.topic_id = topics.id").
		Joins("JOIN tags ON tags.id = topic_tags.tag_id").
		Where("tags.name = ?", normalizeTag(tagName)).
		Count(&count).Error
	return count, err
}

func normalizeTag(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ToLower(name)
	return name
}
