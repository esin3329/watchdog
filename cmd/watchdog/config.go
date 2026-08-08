package main

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Target: 점검 대상 HTTP 서비스
type Target struct {
	Name     string `yaml:"name"`
	URL      string `yaml:"url"`
	Insecure bool   `yaml:"insecure"` // 자체 서명 인증서 허용
}

// DiscordConfig: Discord Webhook 설정
type DiscordConfig struct {
	WebhookURL string `yaml:"webhook_url"`
}

// Config: 전체 설정
type Config struct {
	CheckInterval     time.Duration `yaml:"check_interval"`
	HTTPTimeout       time.Duration `yaml:"http_timeout"`
	FailureThreshold  int           `yaml:"failure_threshold"`
	RecoveryThreshold int           `yaml:"recovery_threshold"`
	StateFile         string        `yaml:"state_file"`
	Targets           []Target      `yaml:"targets"`
	Discord           DiscordConfig `yaml:"discord"`
}

const (
	defaultCheckInterval     = 60 * time.Second
	defaultHTTPTimeout       = 10 * time.Second
	defaultFailureThreshold  = 3
	defaultRecoveryThreshold = 2
	defaultStateFile         = "state.json"
)

// loadConfig: 설정 파일을 읽고 기본값을 적용한다.
func loadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("설정 파일 읽기 실패: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("설정 파일 파싱 실패: %w", err)
	}

	cfg.applyDefaults()
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// applyDefaults: 설정에 없는 항목은 기본값을 채운다.
func (c *Config) applyDefaults() {
	if c.CheckInterval == 0 {
		c.CheckInterval = defaultCheckInterval
	}
	if c.HTTPTimeout == 0 {
		c.HTTPTimeout = defaultHTTPTimeout
	}
	if c.FailureThreshold == 0 {
		c.FailureThreshold = defaultFailureThreshold
	}
	if c.RecoveryThreshold == 0 {
		c.RecoveryThreshold = defaultRecoveryThreshold
	}
	if c.StateFile == "" {
		c.StateFile = defaultStateFile
	}
}

// validate: 설정값 무결성 검사
func (c *Config) validate() error {
	if len(c.Targets) == 0 {
		return fmt.Errorf("targets가 비어 있습니다")
	}
	if c.CheckInterval <= 0 {
		return fmt.Errorf("check_interval은 0보다 커야 합니다")
	}
	if c.HTTPTimeout <= 0 {
		return fmt.Errorf("http_timeout은 0보다 커야 합니다")
	}
	if c.FailureThreshold < 1 {
		return fmt.Errorf("failure_threshold는 1 이상이어야 합니다")
	}
	if c.RecoveryThreshold < 1 {
		return fmt.Errorf("recovery_threshold는 1 이상이어야 합니다")
	}

	seen := make(map[string]bool)
	for _, t := range c.Targets {
		if t.Name == "" {
			return fmt.Errorf("target의 name은 필수입니다")
		}
		if t.URL == "" {
			return fmt.Errorf("target '%s'의 url은 필수입니다", t.Name)
		}
		if seen[t.Name] {
			return fmt.Errorf("target 이름이 중복됩니다: %s", t.Name)
		}
		seen[t.Name] = true
	}
	return nil
}
