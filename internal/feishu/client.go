package feishu

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// Client is a Feishu API client for sending messages.
type Client struct {
	appID     string
	appSecret string
	token     string
	tokenMu   sync.RWMutex
	tokenTime time.Time
}

// NewClient creates a new Feishu client.
func NewClient(appID, appSecret string) *Client {
	return &Client{
		appID:     appID,
		appSecret: appSecret,
	}
}

// getToken returns a valid tenant access token, refreshing if needed.
func (c *Client) getToken() (string, error) {
	c.tokenMu.RLock()
	if c.token != "" && time.Since(c.tokenTime) < 1*time.Hour {
		token := c.token
		c.tokenMu.RUnlock()
		return token, nil
	}
	c.tokenMu.RUnlock()

	// Refresh token
	body := map[string]string{
		"app_id":    c.appID,
		"app_secret": c.appSecret,
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", "https://open.feishu.cn/open-apis/auth/v3/tenant_access_token/internal", bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request token: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Code              int    `json:"code"`
		Msg               string `json:"msg"`
		TenantAccessToken string `json:"tenant_access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}
	if result.Code != 0 {
		return "", fmt.Errorf("token error %d: %s", result.Code, result.Msg)
	}

	c.tokenMu.Lock()
	c.token = result.TenantAccessToken
	c.tokenTime = time.Now()
	c.tokenMu.Unlock()

	return result.TenantAccessToken, nil
}

// SendTextMessage sends a text message to a Feishu chat.
func (c *Client) SendTextMessage(chatID, content string) error {
	token, err := c.getToken()
	if err != nil {
		return err
	}

	payload := map[string]interface{}{
		"receive_id": chatID,
		"msg_type":   "text",
		"content": map[string]string{
			"text": content,
		},
	}
	return c.sendMessage(token, chatID, payload)
}

// SendRichMessage sends a rich message (interactive card) to a Feishu chat.
func (c *Client) SendRichMessage(chatID, title, content string) error {
	token, err := c.getToken()
	if err != nil {
		return err
	}

	// Build card content
	card := map[string]interface{}{
		"config": map[string]interface{}{
			"wide_screen_mode": true,
		},
		"elements": []interface{}{
			map[string]interface{}{
				"tag":  "markdown",
				"content": fmt.Sprintf("**%s**\n\n%s", title, content),
			},
			map[string]interface{}{
				"tag":   "action",
				"layout": "right",
				"actions": []interface{}{
					map[string]interface{}{
						"tag": "button",
						"text": map[string]string{
							"tag":  "plain_text",
							"content": "View topic",
						},
						"type": "primary",
					},
				},
			},
		},
	}

	payload := map[string]interface{}{
		"receive_id": chatID,
		"msg_type":   "interactive",
		"content":    json.RawMessage(`{}`), // filled below
	}

	cardJSON, err := json.Marshal(card)
	if err != nil {
		return fmt.Errorf("marshal card: %w", err)
	}
	payload["content"] = json.RawMessage(cardJSON)

	return c.sendMessage(token, chatID, payload)
}

func (c *Client) sendMessage(token, chatID string, payload map[string]interface{}) error {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	req, err := http.NewRequest("POST", "https://open.feishu.cn/open-apis/im/v1/messages?receive_id_type=chat_id", bytes.NewReader(payloadBytes))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("send message: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	if result.Code != 0 {
		return fmt.Errorf("send error %d: %s", result.Code, result.Msg)
	}
	return nil
}
