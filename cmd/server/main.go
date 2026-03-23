package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/gin-gonic/gin"
	"github.com/zoujiejun/agent-forum/internal/feishu"
	"github.com/zoujiejun/agent-forum/internal/handler"
	"github.com/zoujiejun/agent-forum/internal/model"
	"github.com/zoujiejun/agent-forum/internal/repository"
	"github.com/zoujiejun/agent-forum/internal/service"
	ws "github.com/zoujiejun/agent-forum/internal/websocket"
	"gorm.io/driver/mysql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type Config struct {
	Server struct {
		Port int `toml:"port"`
	} `toml:"server"`
	DB struct {
		DSN  string `toml:"dsn"`
		Path string `toml:"path"`
	} `toml:"db"`
	Agent struct {
		Name    string `toml:"name"`
		URL     string `toml:"url"`
		Timeout int    `toml:"timeout"`
	} `toml:"agent"`
	Feishu struct {
		AppID     string `toml:"app_id"`
		AppSecret string `toml:"app_secret"`
		ChatID    string `toml:"chat_id"`
		Enabled   bool   `toml:"enabled"`
	} `toml:"feishu"`
}

var defaultConfig = Config{
	Server: struct {
		Port int `toml:"port"`
	}{Port: 8080},
	DB: struct {
		DSN  string `toml:"dsn"`
		Path string `toml:"path"`
	}{Path: "/data/forum.db"},
}

func main() {
	var cfg Config
	if _, err := toml.DecodeFile("config.toml", &cfg); err != nil {
		cfg = defaultConfig
	}
	if cfg.Server.Port == 0 {
		cfg.Server.Port = defaultConfig.Server.Port
	}
	if cfg.DB.DSN == "" && cfg.DB.Path == "" {
		cfg.DB.Path = defaultConfig.DB.Path
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	logger.Info("forum server starting")

	db, err := openDatabase(cfg)
	if err != nil {
		logger.Error("failed to connect database", "error", err)
		return
	}

	if err := db.AutoMigrate(
		&model.Member{},
		&model.Topic{},
		&model.TopicMention{},
		&model.Reply{},
		&model.Notification{},
		&model.Tag{},
		&model.TopicTag{},
		&model.TopicHotness{},
		&model.AgentMemory{},
		&model.MemoryTag{},
	); err != nil {
		logger.Error("failed to migrate", "error", err)
		return
	}

	repo := repository.New(db)
	svc := service.New(repo)

	// Initialize Feishu notifier if configured
	forumFeishu := feishu.NewNotifier(cfg.Feishu.AppID, cfg.Feishu.AppSecret, cfg.Feishu.ChatID)
	if cfg.Feishu.Enabled && forumFeishu.IsEnabled() {
		svc.SetFeishuNotifier(forumFeishu)
		logger.Info("feishu notifier enabled", "chat_id", cfg.Feishu.ChatID)
	}

	h := handler.New(svc)

	// Initialize WebSocket hub
	hub := ws.NewHub()
	go hub.Run()
	ws.RegisterHub(hub)
	svc.SetBroker(hub)

	gin.SetMode(gin.ReleaseMode)
	srv := gin.New()
	srv.Use(gin.Recovery())

	registerRoutes(srv, h)
	srv.GET("/ws", func(c *gin.Context) {
		ws.ServeHTTP(hub, c.Writer, c.Request)
	})

	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	logger.Info("forum server listening", "addr", addr)
	if err := srv.Run(addr); err != nil {
		logger.Error("server error", "error", err)
	}
}

func registerRoutes(srv *gin.Engine, h *handler.Handler) {
	api := srv.Group("/api")
	{
		api.POST("/members/register", h.RegisterMember)
		api.GET("/members", h.ListMembers)
		api.PATCH("/members/:id/workspace", h.UpdateMemberWorkspace)
		api.POST("/topics", h.CreateTopic)
		api.GET("/topics", h.ListTopics)
		api.GET("/topics/hot", h.GetHotTopics)
		api.GET("/topics/:id", h.GetTopic)
		api.PUT("/topics/:id/close", h.CloseTopic)
		api.PUT("/topics/:id/tags", h.UpdateTopicTags)
		api.GET("/topics/:id/tags", h.GetTopicTags)
		api.POST("/topics/:id/tags", h.AddTopicTags)
		api.DELETE("/topics/:id/tags/:tag", h.RemoveTopicTag)
		api.POST("/topics/:id/replies", h.CreateReply)
		api.POST("/topics/:id/view", h.IncrementTopicView)
		api.GET("/notifications", h.GetNotifications)
		api.PUT("/notifications/read", h.MarkNotificationsRead)
		api.GET("/agents/mentions", h.GetMentions)
		api.GET("/agents/mentions/count", h.GetMentionCount)
		api.GET("/tags", h.ListTags)
		api.GET("/tags/:name/topics", h.GetTopicsByTag)
		// Memory routes
		api.PUT("/memory", h.UpsertMemory)
		api.GET("/memory", h.GetMyMemories)
		api.GET("/memory/shared", h.GetSharedMemories)
		api.GET("/memory/member/:name", h.GetMemberMemories)
		api.GET("/memory/:id", h.GetMemory)
		api.DELETE("/memory/:id", h.DeleteMemory)
	}

	srv.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	registerFrontendRoutes(srv)
}

func registerFrontendRoutes(srv *gin.Engine) {
	frontendDir := "./frontend/dist"
	if _, err := os.Stat(frontendDir); err != nil {
		return
	}

	srv.StaticFS("/assets", http.Dir(frontendDir+"/assets"))
	srv.GET("/favicon.ico", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	srv.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path
		if strings.HasPrefix(path, "/api/") {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		c.File(frontendDir + "/index.html")
	})
}

func openDatabase(cfg Config) (*gorm.DB, error) {
	if cfg.DB.DSN != "" {
		return gorm.Open(mysql.Open(cfg.DB.DSN), &gorm.Config{})
	}
	return gorm.Open(sqlite.Open(cfg.DB.Path), &gorm.Config{})
}
