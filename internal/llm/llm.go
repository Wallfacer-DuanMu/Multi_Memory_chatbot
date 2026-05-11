package llm

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"whale/internal/config"
)

type Client struct {
	Provider string
	BaseURL  string
	APIKey   string
	Model    string
	HTTP     *http.Client
}

type ReplyRequest struct {
	SystemPrompt string
	Context      string
	UserInput    string
}

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	Temperature float64       `json:"temperature,omitempty"`
}

type ChatResponse struct {
	Choices []struct {
		Message ChatMessage `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type,omitempty"`
	} `json:"error,omitempty"`
}

func NewClientFromConfig(cfg config.LLMConfig) *Client {
	provider := strings.TrimSpace(cfg.Provider)
	if provider == "" {
		provider = "gpt-5.2-codex-compatible"
	}
	return &Client{
		Provider: provider,
		BaseURL:  strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/"),
		APIKey:   strings.TrimSpace(cfg.APIKey),
		Model:    strings.TrimSpace(cfg.Model),
		HTTP:     &http.Client{Timeout: 60 * time.Second},
	}
}

func (c *Client) GenerateReply(req ReplyRequest) (string, error) {
	if strings.TrimSpace(req.UserInput) == "" {
		return "请先输入一点内容，我再继续陪你聊。", nil
	}
	if c == nil || strings.TrimSpace(c.APIKey) == "" || strings.TrimSpace(c.BaseURL) == "" || strings.TrimSpace(c.Model) == "" {
		return fallbackReply(req, errors.New("missing LLM config")), errors.New("missing LLM config")
	}
	reply, err := c.chat(req)
	if err != nil {
		return fallbackReply(req, err), err
	}
	return reply, nil
}

func (c *Client) chat(req ReplyRequest) (string, error) {
	messages := []ChatMessage{{Role: "system", Content: strings.TrimSpace(req.SystemPrompt)}}
	if ctx := strings.TrimSpace(req.Context); ctx != "" {
		messages = append(messages, ChatMessage{Role: "user", Content: "以下是可参考的长期记忆与上下文：\n" + ctx})
	}
	messages = append(messages, ChatMessage{Role: "user", Content: req.UserInput})
	payload := ChatRequest{Model: c.Model, Messages: messages, Temperature: 0.7}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	reqHTTP, err := http.NewRequest(http.MethodPost, c.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	reqHTTP.Header.Set("Content-Type", "application/json")
	reqHTTP.Header.Set("Authorization", "Bearer "+c.APIKey)
	resp, err := c.HTTP.Do(reqHTTP)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var decoded ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return "", err
	}
	if resp.StatusCode >= 300 {
		if decoded.Error != nil && decoded.Error.Message != "" {
			return "", fmt.Errorf("provider returned %d: %s", resp.StatusCode, decoded.Error.Message)
		}
		return "", fmt.Errorf("provider returned %d", resp.StatusCode)
	}
	if len(decoded.Choices) == 0 {
		return "", errors.New("empty chat response choices")
	}
	content := strings.TrimSpace(decoded.Choices[0].Message.Content)
	if content == "" {
		return "", errors.New("empty assistant content")
	}
	return content, nil
}

func fallbackReply(req ReplyRequest, cause error) string {
	base := "抱歉，刚刚模型调用没有成功，所以这次先由系统兜底回复。"
	if cause != nil {
		base += " 原因：" + cause.Error() + "。"
	}
	if strings.TrimSpace(req.Context) == "" {
		return base + " 我已经记录了你的输入，等模型恢复后会继续按正常链路回复。"
	}
	return base + " 我也已经结合现有记忆保留了上下文，等模型恢复后会继续按正常链路回复。"
}
