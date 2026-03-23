package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/zoujiejun/agent-forum/internal/model"
	"github.com/zoujiejun/agent-forum/internal/repository"
	"github.com/zoujiejun/agent-forum/internal/service"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newTestRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	dsn := "file:handler_test_" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Member{}, &model.Topic{}, &model.TopicMention{}, &model.Reply{}, &model.Notification{}, &model.Tag{}, &model.TopicTag{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	repo := repository.New(db)
	svc := service.New(repo)
	h := New(svc)

	r := gin.New()
	h.RegisterRoutes(r)
	return r
}

func registerMemberViaAPI(t *testing.T, router *gin.Engine, name string) {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"name": name, "workspace": "workspace-test"})
	req := httptest.NewRequest(http.MethodPost, "/api/members/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("register member %s failed: %d %s", name, w.Code, w.Body.String())
	}
}

func TestCreateTopicWithoutHeaderReturnsBadRequest(t *testing.T) {
	router := newTestRouter(t)
	body := []byte(`{"title":"Test Title","content":"Test Content"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/topics", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d, body=%s", w.Code, w.Body.String())
	}
}

func TestCloseTopicUnauthorizedReturnsForbidden(t *testing.T) {
	router := newTestRouter(t)
	registerMemberViaAPI(t, router, "agent-alpha")
	registerMemberViaAPI(t, router, "agent-beta")

	body := []byte(`{"title":"Test Title","content":"Test Content"}`)
	createReq := httptest.NewRequest(http.MethodPost, "/api/topics", bytes.NewReader(body))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("X-Agent-Name", "agent-alpha")
	createW := httptest.NewRecorder()
	router.ServeHTTP(createW, createReq)
	if createW.Code != http.StatusOK {
		t.Fatalf("create topic failed: %d %s", createW.Code, createW.Body.String())
	}

	closeReq := httptest.NewRequest(http.MethodPut, "/api/topics/1/close", bytes.NewReader([]byte(`{}`)))
	closeReq.Header.Set("Content-Type", "application/json")
	closeReq.Header.Set("X-Agent-Name", "agent-beta")
	closeW := httptest.NewRecorder()
	router.ServeHTTP(closeW, closeReq)

	if closeW.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d, body=%s", closeW.Code, closeW.Body.String())
	}
}

func TestReplyToClosedTopicReturnsConflict(t *testing.T) {
	router := newTestRouter(t)
	registerMemberViaAPI(t, router, "agent-alpha")
	registerMemberViaAPI(t, router, "agent-beta")

	createBody := []byte(`{"title":"Test Title","content":"Test Content"}`)
	createReq := httptest.NewRequest(http.MethodPost, "/api/topics", bytes.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("X-Agent-Name", "agent-alpha")
	createW := httptest.NewRecorder()
	router.ServeHTTP(createW, createReq)
	if createW.Code != http.StatusOK {
		t.Fatalf("create topic failed: %d %s", createW.Code, createW.Body.String())
	}

	closeReq := httptest.NewRequest(http.MethodPut, "/api/topics/1/close", bytes.NewReader([]byte(`{}`)))
	closeReq.Header.Set("Content-Type", "application/json")
	closeReq.Header.Set("X-Agent-Name", "agent-alpha")
	closeW := httptest.NewRecorder()
	router.ServeHTTP(closeW, closeReq)
	if closeW.Code != http.StatusOK {
		t.Fatalf("close topic failed: %d %s", closeW.Code, closeW.Body.String())
	}

	replyReq := httptest.NewRequest(http.MethodPost, "/api/topics/1/replies", bytes.NewReader([]byte(`{"content":"Acknowledged"}`)))
	replyReq.Header.Set("Content-Type", "application/json")
	replyReq.Header.Set("X-Agent-Name", "agent-beta")
	replyW := httptest.NewRecorder()
	router.ServeHTTP(replyW, replyReq)

	if replyW.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d, body=%s", replyW.Code, replyW.Body.String())
	}
}
