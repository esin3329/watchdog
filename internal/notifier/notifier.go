// Package notifier: Discord Webhook 알림 전송.
package notifier

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Notifier: Discord Webhook으로 메시지를 보낸다.
type Notifier struct {
	webhookURL string
	client     *http.Client
}

// New: Notifier 생성. webhookURL이 비어 있으면 Send는 no-op이 된다.
func New(webhookURL string) *Notifier {
	return &Notifier{
		webhookURL: webhookURL,
		client:     &http.Client{Timeout: 10 * time.Second},
	}
}

// Send: 메시지를 Discord로 보낸다.
// 알림 실패는 에러로 반환하되, 호출부(모니터링 루프)가 멈추지 않도록 한다.
func (n *Notifier) Send(content string) error {
	if n.webhookURL == "" {
		return nil // webhook 미설정 — 전송 생략
	}

	payload, err := json.Marshal(map[string]string{"content": content})
	if err != nil {
		return fmt.Errorf("payload 생성 실패: %w", err)
	}

	resp, err := n.client.Post(n.webhookURL, "application/json", bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("webhook 전송 실패: %w", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	if resp.StatusCode >= 400 {
		return fmt.Errorf("discord webhook 응답 %d", resp.StatusCode)
	}
	return nil
}
