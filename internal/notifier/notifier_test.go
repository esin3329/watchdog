package notifier

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSendDeliversPayload(t *testing.T) {
	var received string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("POST가 아님: %s", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); !strings.Contains(ct, "application/json") {
			t.Errorf("Content-Type 이상: %s", ct)
		}
		body, _ := io.ReadAll(r.Body)
		received = string(body)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	n := New(server.URL)
	if err := n.Send("🔴 test message"); err != nil {
		t.Fatalf("Send: %v", err)
	}

	var payload map[string]string
	if err := json.Unmarshal([]byte(received), &payload); err != nil {
		t.Fatalf("payload 파싱 실패: %v (%s)", err, received)
	}
	if payload["content"] != "🔴 test message" {
		t.Fatalf("content가 다름: %q", payload["content"])
	}
}

func TestSendErrorOnNonSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	n := New(server.URL)
	if err := n.Send("test"); err == nil {
		t.Fatal("400 응답인데 에러가 없음")
	}
}

func TestSendNoopWhenWebhookEmpty(t *testing.T) {
	n := New("")
	if err := n.Send("test"); err != nil {
		t.Fatalf("webhook 미설정 Send는 nil이어야 함: %v", err)
	}
}
