package chat_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/mudler/nib/chat"
	"github.com/mudler/nib/types"
	"github.com/mudler/xlog"
)

// TestInterruptPreservesPriorHistory proves that interrupting a turn (Ctrl+C /
// context canceled) does not destroy the conversation history from earlier
// completed turns. The session must remain usable with full memory of what
// was already said.
func TestInterruptPreservesPriorHistory(t *testing.T) {
	xlog.SetLogger(xlog.NewLogger(xlog.LogLevel("error"), ""))

	// Second request blocks until released, so we can interrupt it mid-flight.
	release := make(chan struct{})
	secondStarted := make(chan struct{}, 1)
	var mu sync.Mutex
	var secondMessages []capturedMessage
	_ = secondMessages // captured for debugging

	mux := http.NewServeMux()
	var callCount int
	mux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 2 {
			body, _ := readAll(r)
			var req struct {
				Messages []capturedMessage `json:"messages"`
			}
			_ = json.Unmarshal(body, &req)
			mu.Lock()
			secondMessages = req.Messages
			mu.Unlock()

			select {
			case secondStarted <- struct{}{}:
			default:
			}
			select {
			case <-release:
			case <-r.Context().Done():
				return
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "fake", "object": "chat.completion", "model": "fake",
			"choices": []any{map[string]any{
				"index": 0, "message": map[string]any{"role": "assistant", "content": "ok"},
				"finish_reason": "stop",
			}},
		})
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	cfg := types.Config{
		Model:        "fake-model",
		APIKey:       "fake-key",
		BaseURL:      srv.URL + "/v1",
		LogLevel:     "error",
		ApprovalMode: "auto",
		AgentOptions: types.AgentOptions{Iterations: 10, MaxAttempts: 3, MaxRetries: 3},
	}

	session, err := chat.NewSession(context.Background(), cfg, chat.Callbacks{})
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	defer session.Close()

	// Turn 1: completes normally.
	if _, err := session.SendMessage("remember the number 42"); err != nil {
		t.Fatalf("first SendMessage: %v", err)
	}

	// Turn 2: will be interrupted mid-flight.
	go func() {
		<-secondStarted
		time.Sleep(50 * time.Millisecond)
		session.Interrupt()
	}()

	_, err2 := session.SendMessage("what number did I tell you?")
	if err2 == nil {
		close(release)
		t.Fatalf("second SendMessage should have been interrupted, but succeeded")
	}
	close(release)

	// The interrupted turn must not have destroyed the first turn.
	exported := session.ExportHistory()
	t.Logf("ExportHistory after interrupt (%d msgs):", len(exported))
	for i, m := range exported {
		t.Logf("  [%d] %s: %s", i, m.Role, m.Content)
	}

	hasFirstUser := false
	hasFirstAssistant := false
	for _, m := range exported {
		if m.Role == "user" && m.Content == "remember the number 42" {
			hasFirstUser = true
		}
		if m.Role == "assistant" && m.Content == "ok" {
			hasFirstAssistant = true
		}
	}
	if !hasFirstUser {
		t.Errorf("interrupt destroyed history: first user message missing from ExportHistory")
	}
	if !hasFirstAssistant {
		t.Errorf("interrupt destroyed history: first assistant reply missing from ExportHistory")
	}

	// Turn 3: the session must still be usable — the model must see the first turn.
	var thirdMessages []capturedMessage
	srv2 := httptest.NewServer(messageCapturingOpenAI(func(m []capturedMessage) {
		thirdMessages = m
	}))
	defer srv2.Close()

	cfg2 := cfg
	cfg2.BaseURL = srv2.URL + "/v1"
	cfg2.InitialHistory = exported
	session2, err := chat.NewSession(context.Background(), cfg2, chat.Callbacks{})
	if err != nil {
		t.Fatalf("NewSession for turn 3: %v", err)
	}
	defer session2.Close()

	if _, err := session2.SendMessage("what number?"); err != nil {
		t.Fatalf("third SendMessage: %v", err)
	}

	found := false
	for _, m := range thirdMessages {
		if m.Role == "user" && m.Content == "remember the number 42" {
			found = true
		}
	}
	if !found {
		t.Errorf("model did not see the first turn after interrupt; messages seen:\n%s", formatMsgs(thirdMessages))
	}
}

func formatMsgs(msgs []capturedMessage) string {
	var s string
	for i, m := range msgs {
		s += fmt.Sprintf("  [%d] %s: %s\n", i, m.Role, m.Content)
	}
	return s
}
