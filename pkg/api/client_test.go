package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestCreateMessageRetriesAndUsesBearerToken(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("Authorization = %q", got)
		}
		if r.URL.Path != "/messages" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if attempts.Add(1) == 1 {
			http.Error(w, "overloaded", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":"msg_1","type":"message","role":"assistant","content":[{"type":"text","text":"ok"}],"model":"test","stop_reason":"end_turn","usage":{"input_tokens":1,"output_tokens":1}}`)
	}))
	defer server.Close()

	client := NewClient(Config{BaseURL: server.URL, AuthToken: "test-token", MaxRetries: 1})
	response, err := client.CreateMessage(context.Background(), MessageRequest{Model: "test", MaxTokens: 16})
	if err != nil {
		t.Fatal(err)
	}
	if attempts.Load() != 2 {
		t.Fatalf("attempts = %d, want 2", attempts.Load())
	}
	if len(response.Content) != 1 || response.Content[0].Text != "ok" {
		t.Fatalf("unexpected response: %#v", response)
	}
}

func TestStreamMessageParsesSSE(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintln(w, "event: content_block_delta")
		fmt.Fprintln(w, `data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"hello"}}`)
		fmt.Fprintln(w)
		fmt.Fprintln(w, "event: message_stop")
		fmt.Fprintln(w, `data: {"type":"message_stop"}`)
	}))
	defer server.Close()

	client := NewClient(Config{BaseURL: server.URL, APIKey: "test-key"})
	var events []StreamEvent
	err := client.StreamMessage(context.Background(), MessageRequest{Model: "test", MaxTokens: 16}, func(event StreamEvent) error {
		events = append(events, event)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Delta == nil || events[0].Delta.Text != "hello" {
		t.Fatalf("unexpected events: %#v", events)
	}
}
