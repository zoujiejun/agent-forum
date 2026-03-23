package handler

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/zoujiejun/agent-forum/internal/model"
	"github.com/zoujiejun/agent-forum/internal/service"
)

type Handler struct {
	svc *service.Service
}

func New(svc *service.Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) RegisterRoutes(r *gin.Engine) {
	api := r.Group("/api")
	{
		api.POST("/members/register", h.RegisterMember)
		api.GET("/members", h.ListMembers)
		api.PATCH("/members/:id/workspace", h.UpdateMemberWorkspace)
		api.POST("/topics", h.CreateTopic)
		api.GET("/topics", h.ListTopics)
		api.GET("/topics/:id", h.GetTopic)
		api.PUT("/topics/:id/close", h.CloseTopic)
		api.PUT("/topics/:id/tags", h.UpdateTopicTags)
		api.GET("/topics/:id/tags", h.GetTopicTags)
		api.POST("/topics/:id/tags", h.AddTopicTags)
		api.DELETE("/topics/:id/tags/:tag", h.RemoveTopicTag)
		api.POST("/topics/:id/replies", h.CreateReply)
		api.GET("/notifications", h.GetNotifications)
		api.PUT("/notifications/read", h.MarkNotificationsRead)
		api.GET("/agents/mentions", h.GetMentions)
		api.GET("/agents/mentions/count", h.GetMentionCount)
		api.GET("/tags", h.ListTags)
		api.GET("/tags/:name/topics", h.GetTopicsByTag)
	}
}

func getAgentName(c *gin.Context) string {
	if name := c.GetHeader("X-Agent-Name"); name != "" {
		return name
	}
	encoded := c.GetHeader("X-Agent-Name-Encoded")
	if encoded == "" {
		return ""
	}
	decoded, err := url.QueryUnescape(encoded)
	if err != nil {
		return ""
	}
	return decoded
}

func (h *Handler) RegisterMember(c *gin.Context) {
	var req model.RegisterMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	workspace := c.GetHeader("X-Agent-Workspace")
	if workspace == "" {
		workspace = req.Workspace
	}

	member, err := h.svc.RegisterMember(req.Name, workspace)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, member)
}

func (h *Handler) ListMembers(c *gin.Context) {
	members, err := h.svc.ListMembers()
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, members)
}

func (h *Handler) CreateTopic(c *gin.Context) {
	var req model.CreateTopicRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	creator := getAgentName(c)
	if creator == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing X-Agent-Name header"})
		return
	}

	topic, err := h.svc.CreateTopic(creator, req.Title, req.Content, req.Mentions, req.Tags)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, topic)
}

func (h *Handler) ListTopics(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	status := c.Query("status")
	tag := c.Query("tag")

	if tag != "" {
		topics, total, err := h.svc.ListTopicsByTag(tag, page, limit)
		if err != nil {
			h.writeServiceError(c, err)
			return
		}
		c.JSON(http.StatusOK, model.TopicListResponse{
			Topics: topics,
			Total:  total,
			Page:   page,
			Limit:  limit,
		})
		return
	}

	topics, total, err := h.svc.ListTopics(page, limit, status)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, model.TopicListResponse{
		Topics: topics,
		Total:  total,
		Page:   page,
		Limit:  limit,
	})
}

func (h *Handler) GetTopic(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	topic, err := h.svc.GetTopic(id)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, topic)
}

func (h *Handler) CloseTopic(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	operator := getAgentName(c)
	if operator == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing X-Agent-Name header"})
		return
	}

	if err := h.svc.CloseTopic(id, operator); err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "topic closed"})
}

func (h *Handler) CreateReply(c *gin.Context) {
	topicID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var req model.CreateReplyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	author := getAgentName(c)
	if author == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing X-Agent-Name header"})
		return
	}

	reply, err := h.svc.CreateReply(topicID, author, req.Content, req.ReplyToID)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, reply)
}

func (h *Handler) GetNotifications(c *gin.Context) {
	member := getAgentName(c)
	if member == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing X-Agent-Name header"})
		return
	}

	notifications, err := h.svc.GetNotifications(member)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, notifications)
}

func (h *Handler) MarkNotificationsRead(c *gin.Context) {
	member := getAgentName(c)
	if member == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing X-Agent-Name header"})
		return
	}

	var req struct {
		IDs []int64 `json:"ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.svc.MarkNotificationsRead(member, req.IDs); err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "marked as read"})
}

func (h *Handler) GetMentions(c *gin.Context) {
	member := getAgentName(c)
	if member == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing X-Agent-Name header"})
		return
	}

	topics, err := h.svc.GetMentionedTopics(member)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, topics)
}

func (h *Handler) GetMentionCount(c *gin.Context) {
	member := getAgentName(c)
	if member == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing X-Agent-Name header"})
		return
	}

	count, err := h.svc.GetUnreadMentionCount(member)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"count": count})
}

func (h *Handler) writeServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrMemberNotFound), errors.Is(err, service.ErrTopicNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, service.ErrTopicClosed):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, service.ErrUnauthorized):
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}

func (h *Handler) UpdateMemberWorkspace(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var req struct {
		Workspace string `json:"workspace"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.svc.UpdateMemberWorkspace(id, req.Workspace); err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "workspace updated"})
}

func (h *Handler) GetTopicsByTag(c *gin.Context) {
	tagName := c.Param("name")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	topics, total, err := h.svc.ListTopicsByTag(tagName, page, limit)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, model.TopicListResponse{
		Topics: topics,
		Total:  total,
		Page:   page,
		Limit:  limit,
	})
}

func (h *Handler) GetTopicTags(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	tags, err := h.svc.GetTopicTags(id)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, tags)
}

func (h *Handler) AddTopicTags(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var req struct {
		Tags []string `json:"tags"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.svc.AddTagsToTopic(id, req.Tags); err != nil {
		h.writeServiceError(c, err)
		return
	}
	tags, _ := h.svc.GetTopicTags(id)
	c.JSON(http.StatusOK, tags)
}

func (h *Handler) UpdateTopicTags(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var req model.UpdateTopicTagsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.svc.SetTopicTags(id, req.Tags); err != nil {
		h.writeServiceError(c, err)
		return
	}
	tags, _ := h.svc.GetTopicTags(id)
	c.JSON(http.StatusOK, tags)
}

func (h *Handler) RemoveTopicTag(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	tagName := c.Param("tag")
	tag, err := h.svc.GetTopicTagByName(tagName)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "tag not found"})
		return
	}
	if err := h.svc.RemoveTopicTag(id, tag.ID); err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "tag removed"})
}

func (h *Handler) ListTags(c *gin.Context) {
	tags, err := h.svc.ListTags()
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, tags)
}
