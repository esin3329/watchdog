// watchdog: 미니 PC와 Docker 서비스 상태를 주기적으로 점검하고
// 상태 변화 시 Discord Webhook으로 알림을 보내는 경량 모니터링 프로그램.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"watchdog/internal/checker"
	"watchdog/internal/notifier"
	"watchdog/internal/state"
)

func main() {
	configPath := flag.String("config", "config.yaml", "설정 파일 경로")
	flag.Parse()

	cfg, err := loadConfig(*configPath)
	if err != nil {
		log.Fatalf("[FATAL] 설정 로드 실패: %v", err)
	}

	log.Printf("[START] watchdog 시작 — 대상 %d개, 체크 주기 %s, 타임아웃 %s",
		len(cfg.Targets), cfg.CheckInterval, cfg.HTTPTimeout)
	if cfg.Discord.WebhookURL == "" {
		log.Printf("[WARN] discord.webhook_url이 설정되지 않아 알림을 보내지 않습니다")
	}

	// 상태 저장소 (재시작 후에도 상태 유지)
	store := state.New(cfg.StateFile, cfg.FailureThreshold, cfg.RecoveryThreshold)
	if err := store.Load(); err != nil {
		log.Printf("[WARN] 상태 파일 로드 실패 (초기 상태로 시작): %v", err)
	}

	ck := checker.New(cfg.HTTPTimeout)
	nf := notifier.New(cfg.Discord.WebhookURL)

	// SIGINT(SIGTERM) 수신 시 context 취소 → graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// 첫 사이클은 대기 없이 즉시 실행
	runCycle(cfg, store, ck, nf)

	ticker := time.NewTicker(cfg.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			runCycle(cfg, store, ck, nf)
		case <-ctx.Done():
			log.Printf("[SHUTDOWN] 종료 신호 수신 — 상태 저장 후 종료")
			if err := store.Save(); err != nil {
				log.Printf("[WARN] 상태 저장 실패: %v", err)
			}
			return
		}
	}
}

// targetResult: 병렬 체크 결과를 인덱스로 모으기 위한 묶음
type targetResult struct {
	target Target
	result checker.Result
}

// runCycle: 모든 대상을 goroutine으로 병렬 체크하고, 상태 반영 + 알림 + 상태 저장을 수행한다.
func runCycle(cfg *Config, store *state.Store, ck *checker.Checker, nf *notifier.Notifier) {
	results := make([]targetResult, len(cfg.Targets))

	var wg sync.WaitGroup
	for i, t := range cfg.Targets {
		wg.Add(1)
		go func(i int, t Target) {
			defer wg.Done()
			results[i] = targetResult{target: t, result: ck.Check(t.URL, t.Insecure)}
		}(i, t)
	}
	wg.Wait() // 모든 체크가 끝날 때까지 대기 (각 체크는 timeout에 의해 보장)

	now := time.Now()
	for _, tr := range results {
		prev, curr, notify := store.RecordResult(tr.target.Name, tr.result.OK, now)

		status := "—"
		if curr != "" {
			status = string(curr)
		}
		log.Printf("[CHECK] %s: %s (%s, %s)",
			tr.target.Name, status, tr.result.Detail, tr.result.Latency.Round(time.Millisecond))

		if notify {
			message := buildMessage(tr.target, tr.result, prev, curr, now)
			log.Printf("[ALERT] %s: %s → %s — Discord 알림 전송", tr.target.Name, prev, curr)
			if err := nf.Send(message); err != nil {
				log.Printf("[WARN] Discord 알림 전송 실패 (%s): %v", tr.target.Name, err)
			}
		}
	}

	if err := store.Save(); err != nil {
		log.Printf("[WARN] 상태 파일 저장 실패: %v", err)
	}
}

// buildMessage: Discord 알림 메시지 생성
func buildMessage(t Target, r checker.Result, prev, curr state.Status, now time.Time) string {
	timestamp := now.Format("2006-01-02 15:04:05")
	line := fmt.Sprintf("`%s` %s | %s", t.URL, r.Detail, r.Latency.Round(time.Millisecond))

	switch {
	case curr == state.StatusDown:
		return fmt.Sprintf("🔴 **DOWN — %s**\n%s\n시각: %s", t.Name, line, timestamp)
	case curr == state.StatusUp && prev == state.StatusDown:
		return fmt.Sprintf("🟢 **UP — %s**\n%s\n시각: %s", t.Name, line, timestamp)
	default:
		return fmt.Sprintf("⚪ **%s — %s**\n%s\n시각: %s", curr, t.Name, line, timestamp)
	}
}
