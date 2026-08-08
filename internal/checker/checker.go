// Package checker: HTTP 헬스 체크.
package checker

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Result: 체크 한 건의 결과
type Result struct {
	OK      bool          // 정상 응답 여부 (HTTP 2xx~3xx)
	Detail  string        // 사람이 읽을 수 있는 상세 (상태 코드 또는 에러)
	Latency time.Duration // 응답까지 걸린 시간
}

// Checker: HTTP 헬스 체크 수행자.
// 기본 client와 자체 서명 인증서 허용 client를 따로 둔다.
type Checker struct {
	timeout        time.Duration
	secureClient   *http.Client
	insecureClient *http.Client
}

// New: Checker 생성. timeout은 HTTP 요청 전체에 적용된다.
func New(timeout time.Duration) *Checker {
	secure := &http.Client{Timeout: timeout}

	// 기본 Transport를 복제해 커넥션 풀 기본값을 유지한 채 TLS 설정만 바꾼다.
	insecureTransport := http.DefaultTransport.(*http.Transport).Clone()
	insecureTransport.TLSClientConfig = &tls.Config{
		InsecureSkipVerify: true, // #nosec G402 — config.yaml의 insecure: true로 명시한 대상만 사용
	}
	insecure := &http.Client{Timeout: timeout, Transport: insecureTransport}

	return &Checker{timeout: timeout, secureClient: secure, insecureClient: insecure}
}

// Check: URL 한 곳을 점검한다.
// DNS 조회부터 포함해 timeout을 넘기면 실패로 처리한다 (context.WithTimeout).
func (c *Checker) Check(url string, insecure bool) Result {
	client := c.secureClient
	if insecure {
		client = c.insecureClient
	}

	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Result{OK: false, Detail: "요청 생성 실패: " + err.Error(), Latency: time.Since(start)}
	}

	resp, err := client.Do(req)
	if err != nil {
		return Result{OK: false, Detail: err.Error(), Latency: time.Since(start)}
	}
	defer resp.Body.Close()

	// 커넥션 재사용을 위해 본문은 소량만 소비한다.
	io.Copy(io.Discard, io.LimitReader(resp.Body, 1024))

	ok := resp.StatusCode >= 200 && resp.StatusCode < 400
	return Result{
		OK:      ok,
		Detail:  fmt.Sprintf("HTTP %d", resp.StatusCode),
		Latency: time.Since(start),
	}
}
