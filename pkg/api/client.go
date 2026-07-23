package api

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Client is the Anthropic API client.
type Client struct {
	apiKey     string
	authToken  string
	baseURL    string
	httpClient *http.Client
	maxRetries int
}

// Config contains client configuration.
type Config struct {
	APIKey     string
	AuthToken  string
	BaseURL    string
	MaxRetries int
	Timeout    time.Duration
	MaxTokens  int
}

// NewClient creates a new API client.
func NewClient(config Config) *Client {
	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = "https://api.anthropic.com/v1"
	}

	timeout := config.Timeout
	if timeout == 0 {
		timeout = 60 * time.Second
	}
	maxRetries := config.MaxRetries
	if maxRetries == 0 {
		maxRetries = 2
	}

	return &Client{
		apiKey:    config.APIKey,
		authToken: config.AuthToken,
		baseURL:   baseURL,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		maxRetries: maxRetries,
	}
}

// MessageRequest represents a message request.
type MessageRequest struct {
	Model       string                 `json:"model"`
	MaxTokens   int                    `json:"max_tokens"`
	Messages    []Message              `json:"messages"`
	System      interface{}            `json:"system,omitempty"`
	Tools       []ToolDefinition       `json:"tools,omitempty"`
	Temperature float64                `json:"temperature,omitempty"`
	Thinking    *ThinkingConfig        `json:"thinking,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	Stream      bool                   `json:"stream,omitempty"`
}

// Message represents a single message.
type Message struct {
	Role    string         `json:"role"`
	Content []ContentBlock `json:"content"`
}

// ContentBlock represents a block of content.
type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`

	// For tool use
	ID        string          `json:"id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	ToolUseID string          `json:"tool_use_id,omitempty"`
	Content   interface{}     `json:"content,omitempty"`
	IsError   bool            `json:"is_error,omitempty"`

	// For thinking
	Thinking string `json:"thinking,omitempty"`
}

// ToolDefinition defines a tool for the API.
type ToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	InputSchema map[string]interface{} `json:"input_schema"`
}

// ThinkingConfig configures thinking mode.
type ThinkingConfig struct {
	Type        string `json:"type"`
	BudgetToken int    `json:"budget_tokens"`
}

// MessageResponse represents the API response.
type MessageResponse struct {
	ID           string         `json:"id"`
	Type         string         `json:"type"`
	Role         string         `json:"role"`
	Content      []ContentBlock `json:"content"`
	Model        string         `json:"model"`
	StopReason   string         `json:"stop_reason"`
	StopSequence string         `json:"stop_sequence,omitempty"`
	Usage        Usage          `json:"usage"`
}

// Usage contains token usage information.
type Usage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens,omitempty"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens,omitempty"`
}

// StreamEvent represents a streaming event.
type StreamEvent struct {
	Type    string           `json:"type"`
	Index   int              `json:"index,omitempty"`
	Delta   *EventDelta      `json:"delta,omitempty"`
	Message *MessageResponse `json:"message,omitempty"`
	Usage   *Usage           `json:"usage,omitempty"`
}

// EventDelta contains the delta for streaming events.
type EventDelta struct {
	Type        string          `json:"type,omitempty"`
	Text        string          `json:"text,omitempty"`
	StopReason  string          `json:"stop_reason,omitempty"`
	Name        string          `json:"name,omitempty"`
	Input       json.RawMessage `json:"input,omitempty"`
	PartialJSON string          `json:"partial_json,omitempty"`
	Thinking    string          `json:"thinking,omitempty"`
}

// CreateMessage sends a message request.
func (c *Client) CreateMessage(ctx context.Context, req MessageRequest) (*MessageResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	for attempt := 0; ; attempt++ {
		httpReq, err := c.newMessageRequest(ctx, body)
		if err != nil {
			return nil, err
		}

		resp, err := c.httpClient.Do(httpReq)
		if err != nil {
			if attempt < c.maxRetries {
				if err := waitForRetry(ctx, retryDelay(attempt, "")); err != nil {
					return nil, err
				}
				continue
			}
			return nil, fmt.Errorf("request failed: %w", err)
		}

		respBody, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("failed to read response: %w", readErr)
		}

		if resp.StatusCode == http.StatusOK {
			var result MessageResponse
			if err := json.Unmarshal(respBody, &result); err != nil {
				return nil, fmt.Errorf("failed to parse response: %w", err)
			}
			return &result, nil
		}

		if attempt < c.maxRetries && (resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500) {
			if err := waitForRetry(ctx, retryDelay(attempt, resp.Header.Get("Retry-After"))); err != nil {
				return nil, err
			}
			continue
		}
		return nil, parseAPIError(resp.Status, respBody)
	}
}

func (c *Client) newMessageRequest(ctx context.Context, body []byte) (*http.Request, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/messages", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.authToken != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.authToken)
	} else if c.apiKey != "" {
		httpReq.Header.Set("x-api-key", c.apiKey)
	}
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	return httpReq, nil
}

func parseAPIError(status string, body []byte) error {
	var apiErr struct {
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &apiErr); err == nil && apiErr.Error.Message != "" {
		return fmt.Errorf("API error: %s - %s", apiErr.Error.Type, apiErr.Error.Message)
	}
	return fmt.Errorf("API error: %s", status)
}

func retryDelay(attempt int, retryAfter string) time.Duration {
	if seconds, err := strconv.Atoi(retryAfter); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	delay := time.Duration(1<<uint(attempt)) * 500 * time.Millisecond
	if delay > 8*time.Second {
		return 8 * time.Second
	}
	return delay
}

func waitForRetry(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// StreamMessage sends a message request with streaming.
func (c *Client) StreamMessage(ctx context.Context, req MessageRequest, onEvent func(event StreamEvent) error) error {
	req.Stream = true

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/messages", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if c.authToken != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.authToken)
	} else if c.apiKey != "" {
		httpReq.Header.Set("x-api-key", c.apiKey)
	}
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error: %s - %s", resp.Status, string(respBody))
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" || data == "[DONE]" {
			continue
		}
		var event StreamEvent
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			return fmt.Errorf("failed to decode stream event: %w", err)
		}
		if event.Type == "message_stop" {
			break
		}
		if err := onEvent(event); err != nil {
			return err
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("failed to read stream: %w", err)
	}

	return nil
}

// CountTokens counts tokens for a message.
func (c *Client) CountTokens(ctx context.Context, req MessageRequest) (int, error) {
	body, err := json.Marshal(map[string]interface{}{
		"model":    req.Model,
		"messages": req.Messages,
	})
	if err != nil {
		return 0, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/messages/count_tokens", bytes.NewReader(body))
	if err != nil {
		return 0, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if c.authToken != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.authToken)
	} else if c.apiKey != "" {
		httpReq.Header.Set("x-api-key", c.apiKey)
	}
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return 0, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		InputTokens int `json:"input_tokens"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("failed to parse response: %w", err)
	}

	return result.InputTokens, nil
}

// SetAPIKey sets the API key.
func (c *Client) SetAPIKey(apiKey string) {
	c.apiKey = apiKey
}

// SetAuthToken sets the bearer token used for OAuth-style authentication.
func (c *Client) SetAuthToken(authToken string) {
	c.authToken = authToken
}

// SetBaseURL sets the base URL.
func (c *Client) SetBaseURL(baseURL string) {
	c.baseURL = baseURL
}
