package checker

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCheckSuccessOn2xx(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	c := New(2 * time.Second)
	result := c.Check(server.URL, false)

	if !result.OK {
		t.Fatalf("200 응답인데 실패: %+v", result)
	}
	if !strings.Contains(result.Detail, "200") {
		t.Fatalf("detail에 상태 코드 없음: %q", result.Detail)
	}
}

func TestCheckFailureOn5xx(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	c := New(2 * time.Second)
	result := c.Check(server.URL, false)

	if result.OK {
		t.Fatalf("500 응답인데 성공 처리: %+v", result)
	}
}

func TestCheckFailureOnConnectionRefused(t *testing.T) {
	// 아무도 안 듣는 포트
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := server.URL
	server.Close() // 즉시 종료 → connection refused

	c := New(2 * time.Second)
	result := c.Check(url, false)

	if result.OK {
		t.Fatalf("연결 거부인데 성공 처리: %+v", result)
	}
}

func TestCheckTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second) // timeout보다 오래 걸림
	}))
	defer server.Close()

	c := New(100 * time.Millisecond)
	result := c.Check(server.URL, false)

	if result.OK {
		t.Fatalf("timeout인데 성공 처리: %+v", result)
	}
	if !strings.Contains(result.Detail, "deadline") && !strings.Contains(result.Detail, "Timeout") {
		t.Fatalf("timeout 상세 메시지가 아님: %q", result.Detail)
	}
	if result.Latency >= 3*time.Second {
		t.Fatalf("timeout이 동작하지 않음 (latency=%s)", result.Latency)
	}
}

func TestCheckInsecureTLS(t *testing.T) {
	// httptest.NewTLSServer는 자체 서명 인증서 사용
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	c := New(2 * time.Second)

	// insecure=false → 인증서 검증 실패
	if result := c.Check(server.URL, false); result.OK {
		t.Fatalf("자체 서명 인증서를 검증 없이 통과함: %+v", result)
	}

	// insecure=true → 통과
	if result := c.Check(server.URL, true); !result.OK {
		t.Fatalf("insecure=true인데 실패: %+v", result)
	}
}
