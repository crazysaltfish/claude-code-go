package query

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"claude-code-go/internal/tools"
	"claude-code-go/internal/types"
	"claude-code-go/pkg/api"
)

func TestQueryEngineToolRoundTrip(t *testing.T) {
	target := filepath.Join(t.TempDir(), "input.txt")
	if err := os.WriteFile(target, []byte("tool output"), 0600); err != nil {
		t.Fatal(err)
	}

	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request api.MessageRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if calls.Add(1) == 1 {
			fmt.Fprintf(w, `{"id":"msg_1","type":"message","role":"assistant","content":[{"type":"tool_use","id":"tool_1","name":"Read","input":{"target_file":%q}}],"model":"test","stop_reason":"tool_use","usage":{"input_tokens":2,"output_tokens":3}}`, target)
			return
		}
		if len(request.Messages) < 3 {
			t.Errorf("second request has %d messages", len(request.Messages))
		} else {
			last := request.Messages[len(request.Messages)-1]
			if last.Role != "user" || len(last.Content) != 1 || last.Content[0].Type != "tool_result" {
				t.Errorf("unexpected tool result message: %#v", last)
			}
		}
		fmt.Fprint(w, `{"id":"msg_2","type":"message","role":"assistant","content":[{"type":"text","text":"final answer"}],"model":"test","stop_reason":"end_turn","usage":{"input_tokens":4,"output_tokens":5}}`)
	}))
	defer server.Close()

	engine := NewQueryEngine(QueryEngineConfig{
		Cwd:       t.TempDir(),
		Tools:     []types.Tool{tools.NewFileReadTool()},
		MaxTurns:  3,
		APIClient: api.NewClient(api.Config{BaseURL: server.URL, APIKey: "test", MaxRetries: 1}),
		CanUseTool: func(context.Context, string, json.RawMessage) (*types.PermissionDecision, error) {
			return &types.PermissionDecision{Behavior: types.PermissionBehaviorAllow}, nil
		},
		GetAppState:        func() *types.AppState { return &types.AppState{} },
		UserSpecifiedModel: "test",
		CustomSystemPrompt: "test",
	})

	output, err := engine.SubmitMessage(context.Background(), "read the file")
	if err != nil {
		t.Fatal(err)
	}
	var result ResultMessage
	var sawToolResult bool
	for event := range output {
		switch event := event.(type) {
		case SDKMessage:
			sawToolResult = sawToolResult || event.Type == "tool_result"
		case ResultMessage:
			result = event
		}
	}

	if !sawToolResult {
		t.Fatal("tool result event was not emitted")
	}
	if result.IsError || result.Result != "final answer" || result.NumTurns != 2 {
		t.Fatalf("unexpected result: %#v", result)
	}
	if result.Usage.InputTokens != 6 || result.Usage.OutputTokens != 8 {
		t.Fatalf("unexpected usage: %#v", result.Usage)
	}
}
