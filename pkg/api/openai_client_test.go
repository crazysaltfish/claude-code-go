package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOpenAIClientConvertsMessagesToolsAndResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer openai-key" {
			t.Errorf("Authorization = %q", got)
		}
		if got := r.Header.Get("OpenAI-Organization"); got != "org-test" {
			t.Errorf("OpenAI-Organization = %q", got)
		}

		var request struct {
			Model    string `json:"model"`
			Messages []struct {
				Role       string           `json:"role"`
				Content    interface{}      `json:"content"`
				ToolCalls  []openAIToolCall `json:"tool_calls"`
				ToolCallID string           `json:"tool_call_id"`
			} `json:"messages"`
			Tools     []openAIToolDefinition `json:"tools"`
			MaxTokens int                    `json:"max_tokens"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		if request.Model != "compatible-model" || request.MaxTokens != 512 {
			t.Errorf("unexpected request settings: %#v", request)
		}
		if len(request.Messages) != 4 {
			t.Fatalf("messages = %d, want 4: %#v", len(request.Messages), request.Messages)
		}
		if request.Messages[0].Role != "system" || request.Messages[0].Content != "system prompt" {
			t.Errorf("unexpected system message: %#v", request.Messages[0])
		}
		if len(request.Messages[2].ToolCalls) != 1 ||
			request.Messages[2].ToolCalls[0].Function.Name != "Read" ||
			request.Messages[2].ToolCalls[0].Function.Arguments != `{"path":"README.md"}` {
			t.Errorf("unexpected assistant tool call: %#v", request.Messages[2])
		}
		if request.Messages[3].Role != "tool" ||
			request.Messages[3].ToolCallID != "call_previous" ||
			request.Messages[3].Content != "file contents" {
			t.Errorf("unexpected tool result message: %#v", request.Messages[3])
		}
		if len(request.Tools) != 1 ||
			request.Tools[0].Type != "function" ||
			request.Tools[0].Function.Name != "Read" {
			t.Errorf("unexpected tools: %#v", request.Tools)
		}

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{
			"id":"chatcmpl_test",
			"object":"chat.completion",
			"model":"compatible-model",
			"choices":[{
				"index":0,
				"message":{
					"role":"assistant",
					"content":"I will inspect it.",
					"tool_calls":[{
						"id":"call_next",
						"type":"function",
						"function":{"name":"Read","arguments":"{\"path\":\"go.mod\"}"}
					}]
				},
				"finish_reason":"tool_calls"
			}],
			"usage":{
				"prompt_tokens":21,
				"completion_tokens":7,
				"prompt_tokens_details":{"cached_tokens":5}
			}
		}`)
	}))
	defer server.Close()

	client := NewOpenAIClient(Config{
		BaseURL:      server.URL,
		APIKey:       "openai-key",
		Organization: "org-test",
	})
	response, err := client.CreateMessage(context.Background(), MessageRequest{
		Model:     "compatible-model",
		MaxTokens: 512,
		System:    "system prompt",
		Messages: []Message{
			{Role: "user", Content: []ContentBlock{{Type: "text", Text: "read the file"}}},
			{Role: "assistant", Content: []ContentBlock{{
				Type:  "tool_use",
				ID:    "call_previous",
				Name:  "Read",
				Input: json.RawMessage(`{"path":"README.md"}`),
			}}},
			{Role: "user", Content: []ContentBlock{{
				Type:      "tool_result",
				ToolUseID: "call_previous",
				Content:   "file contents",
			}}},
		},
		Tools: []ToolDefinition{{
			Name:        "Read",
			Description: "Read a file",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{"type": "string"},
				},
			},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if response.StopReason != "tool_use" {
		t.Fatalf("stop reason = %q", response.StopReason)
	}
	if len(response.Content) != 2 ||
		response.Content[0].Text != "I will inspect it." ||
		response.Content[1].Type != "tool_use" ||
		response.Content[1].ID != "call_next" ||
		response.Content[1].Name != "Read" ||
		string(response.Content[1].Input) != `{"path":"go.mod"}` {
		t.Fatalf("unexpected converted response: %#v", response)
	}
	if response.Usage.InputTokens != 21 ||
		response.Usage.OutputTokens != 7 ||
		response.Usage.CacheReadInputTokens != 5 {
		t.Fatalf("unexpected usage: %#v", response.Usage)
	}
}

func TestOpenAIClientStreamConvertsTextToolCallsAndUsage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		if request["stream"] != true {
			t.Errorf("stream = %#v", request["stream"])
		}
		streamOptions, _ := request["stream_options"].(map[string]interface{})
		if streamOptions["include_usage"] != true {
			t.Errorf("stream_options = %#v", streamOptions)
		}

		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintln(w, `data: {"id":"chatcmpl_stream","model":"test","choices":[{"index":0,"delta":{"content":"hello "},"finish_reason":null}]}`)
		fmt.Fprintln(w)
		fmt.Fprintln(w, `data: {"id":"chatcmpl_stream","model":"test","choices":[{"index":0,"delta":{"content":"world","tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"Read","arguments":"{\"path\":"}}]},"finish_reason":null}]}`)
		fmt.Fprintln(w)
		fmt.Fprintln(w, `data: {"id":"chatcmpl_stream","model":"test","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"\"go.mod\"}"}}]},"finish_reason":"tool_calls"}]}`)
		fmt.Fprintln(w)
		fmt.Fprintln(w, `data: {"id":"chatcmpl_stream","model":"test","choices":[],"usage":{"prompt_tokens":10,"completion_tokens":4}}`)
		fmt.Fprintln(w)
		fmt.Fprintln(w, "data: [DONE]")
	}))
	defer server.Close()

	client := NewOpenAIClient(Config{BaseURL: server.URL, APIKey: "test"})
	var events []StreamEvent
	err := client.StreamMessage(
		context.Background(),
		MessageRequest{Model: "test", MaxTokens: 32},
		func(event StreamEvent) error {
			events = append(events, event)
			return nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	var text, arguments string
	var started bool
	var stopReason string
	var usage *Usage
	for _, event := range events {
		if event.Delta != nil {
			text += event.Delta.Text
			arguments += event.Delta.PartialJSON
			if event.Delta.StopReason != "" {
				stopReason = event.Delta.StopReason
			}
		}
		if event.ContentBlock != nil {
			started = event.ContentBlock.ID == "call_1" && event.ContentBlock.Name == "Read"
		}
		if event.Usage != nil {
			usage = event.Usage
		}
	}
	if text != "hello world" {
		t.Fatalf("streamed text = %q", text)
	}
	if !started {
		t.Fatal("tool call start event was not emitted")
	}
	if arguments != `{"path":"go.mod"}` {
		t.Fatalf("tool arguments = %q", arguments)
	}
	if stopReason != "tool_use" {
		t.Fatalf("stop reason = %q", stopReason)
	}
	if usage == nil || usage.InputTokens != 10 || usage.OutputTokens != 4 {
		t.Fatalf("usage = %#v", usage)
	}
}

func TestOpenAIClientRejectsResponseWithoutChoices(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"id":"empty","object":"chat.completion","choices":[]}`)
	}))
	defer server.Close()

	client := NewOpenAIClient(Config{BaseURL: server.URL})
	_, err := client.CreateMessage(context.Background(), MessageRequest{Model: "test"})
	if err == nil || !strings.Contains(err.Error(), "no choices") {
		t.Fatalf("unexpected error: %v", err)
	}
}

var _ MessageClient = (*OpenAIClient)(nil)
