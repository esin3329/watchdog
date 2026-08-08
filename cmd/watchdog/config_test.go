package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadConfigWithDefaults(t *testing.T) {
	path := writeConfig(t, `
check_interval: 30s
targets:
  - name: Homepage
    url: http://192.168.0.10:3001
  - name: Portainer
    url: https://192.168.0.10:9443
    insecure: true
discord:
  webhook_url: https://discord.com/api/webhooks/test
`)

	cfg, err := loadConfig(path)
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}

	// 명시한 값
	if cfg.CheckInterval != 30*time.Second {
		t.Errorf("check_interval: %s", cfg.CheckInterval)
	}
	if len(cfg.Targets) != 2 {
		t.Errorf("targets 수: %d", len(cfg.Targets))
	}
	if !cfg.Targets[1].Insecure {
		t.Error("Portainer insecure가 false")
	}

	// 기본값
	if cfg.HTTPTimeout != defaultHTTPTimeout {
		t.Errorf("http_timeout 기본값: %s", cfg.HTTPTimeout)
	}
	if cfg.FailureThreshold != defaultFailureThreshold {
		t.Errorf("failure_threshold 기본값: %d", cfg.FailureThreshold)
	}
	if cfg.RecoveryThreshold != defaultRecoveryThreshold {
		t.Errorf("recovery_threshold 기본값: %d", cfg.RecoveryThreshold)
	}
	if cfg.StateFile != defaultStateFile {
		t.Errorf("state_file 기본값: %s", cfg.StateFile)
	}
}

func TestLoadConfigEmptyTargetsFails(t *testing.T) {
	path := writeConfig(t, "check_interval: 10s\n")

	if _, err := loadConfig(path); err == nil {
		t.Fatal("targets가 비어 있는데 에러가 없음")
	}
}

func TestLoadConfigDuplicateTargetNameFails(t *testing.T) {
	path := writeConfig(t, `
targets:
  - name: same
    url: http://a:1
  - name: same
    url: http://b:2
`)

	if _, err := loadConfig(path); err == nil {
		t.Fatal("중복 이름인데 에러가 없음")
	}
}

func TestLoadConfigMissingFileFails(t *testing.T) {
	if _, err := loadConfig(filepath.Join(t.TempDir(), "없는파일.yaml")); err == nil {
		t.Fatal("없는 파일인데 에러가 없음")
	}
}
