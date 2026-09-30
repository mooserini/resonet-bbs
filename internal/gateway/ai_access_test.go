package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"wolfbbs/internal/domain"
)

type memSettings map[string]string

func (m memSettings) GetSystemSetting(key string) (string, error) {
	v, ok := m[key]
	if !ok {
		return "", errors.New("not found")
	}
	return v, nil
}

func (m memSettings) UpsertSystemSetting(key, value string) error {
	m[key] = value
	return nil
}

func TestAIAccessSysopOnlyByDefault(t *testing.T) {
	store := memSettings{}
	now := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	sysop := &domain.User{ID: 1, Handle: "Moose", Role: "sysop"}
	caller := &domain.User{ID: 2, Handle: "Ara", Role: "user"}

	if got := CheckAIAccess(store, sysop, now); !got.Allowed || !got.Unlimited {
		t.Fatalf("sysop should have unlimited access, got %+v", got)
	}
	if got := CheckAIAccess(store, caller, now); got.Allowed {
		t.Fatalf("ungranted user should be denied, got %+v", got)
	}
	if got := CheckAIAccess(nil, caller, now); got.Allowed {
		t.Fatalf("no store should deny non-sysops, got %+v", got)
	}
	if got := CheckAIAccess(store, nil, now); got.Allowed {
		t.Fatalf("anonymous should be denied, got %+v", got)
	}
}

func TestAIAccessGrantAndDailyCap(t *testing.T) {
	store := memSettings{}
	if err := SaveAIPolicy(store, AIPolicy{AllowedHandles: []string{" ARA ", "dogbreath", "ara"}, DailyCap: 2}); err != nil {
		t.Fatalf("save: %v", err)
	}
	if got := store[SettingAIAllowedUsers]; got != "ara, dogbreath" {
		t.Fatalf("handles not normalised: %q", got)
	}
	now := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	caller := &domain.User{ID: 2, Handle: "Ara"}

	for i := 0; i < 2; i++ {
		got := CheckAIAccess(store, caller, now)
		if !got.Allowed || got.Remaining != 2-i {
			t.Fatalf("prompt %d: unexpected access %+v", i, got)
		}
		if err := RecordAIUse(store, caller, now); err != nil {
			t.Fatalf("record: %v", err)
		}
	}
	if got := CheckAIAccess(store, caller, now); got.Allowed {
		t.Fatalf("cap should be reached, got %+v", got)
	}
	if got := CheckAIAccess(store, caller, now.Add(24*time.Hour)); !got.Allowed || got.Remaining != 2 {
		t.Fatalf("cap should reset next day, got %+v", got)
	}

	store[SettingAIDailyCap] = "0"
	if got := CheckAIAccess(store, caller, now); !got.Allowed || !got.Unlimited {
		t.Fatalf("cap 0 should mean no limit, got %+v", got)
	}
}

func TestAIAccessSysopUsageNotCounted(t *testing.T) {
	store := memSettings{}
	sysop := &domain.User{ID: 1, Handle: "moose", Role: "admin"}
	if err := RecordAIUse(store, sysop, time.Now()); err != nil {
		t.Fatalf("record: %v", err)
	}
	if len(store) != 0 {
		t.Fatalf("sysop usage should not be stored: %v", store)
	}
}

func TestAIClientNoThinking(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&payload)
		kwargs, _ := payload["chat_template_kwargs"].(map[string]interface{})
		if kwargs == nil || kwargs["enable_thinking"] != false {
			t.Fatalf("expected enable_thinking=false, got %v", payload["chat_template_kwargs"])
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer server.Close()
	client := NewAIClient(AIConfig{BaseURL: server.URL, AllowPrivate: true, APIKey: "local", Model: "gemma", NoThinking: true})
	if _, err := client.Complete(context.Background(), "hi"); err != nil {
		t.Fatalf("complete: %v", err)
	}
}

func TestAIClientOutOfTokensMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"choices":[{"finish_reason":"length","message":{"content":"","reasoning_content":"hmm"}}]}`))
	}))
	defer server.Close()
	client := NewAIClient(AIConfig{BaseURL: server.URL, AllowPrivate: true, APIKey: "local", Model: "gemma"})
	_, err := client.Complete(context.Background(), "hi")
	if err == nil || !strings.Contains(err.Error(), "Skip thinking") {
		t.Fatalf("expected out-of-tokens hint, got %v", err)
	}
}
