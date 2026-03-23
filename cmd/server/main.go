package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/gin-gonic/gin"
	"github.com/zoujiejun/agent-forum/internal/handler"
	"github.com/zoujiejun/agent-forum/internal/model"
	"github.com/zoujiejun/agent-forum/internal/repository"
	"github.com/zoujiejun/agent-forum/internal/service"
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
	); err != nil {
		logger.Error("failed to migrate", "error", err)
		return
	}

	repo := repository.New(db)
	svc := service.New(repo)
	h := handler.New(svc)

	gin.SetMode(gin.ReleaseMode)
	srv := gin.New()
	srv.Use(gin.Recovery())

	registerRoutes(srv, h)

	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	logger.Info("forum server listening", "addr", addr)
	if err := srv.Run(addr); err != nil {
		logger.Error("server error", "error", err)
	}
}

func registerRoutes(srv *gin.Engine, h *handler.Handler) {
	h.RegisterRoutes(srv)

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
