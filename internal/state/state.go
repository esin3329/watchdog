// Package state: 서비스 상태(UP/DOWN) 관리와 판정 로직.
//
// 판정 규칙 (히스테리시스):
//   - UP 상태에서 연속 failureThreshold회 실패 → DOWN (장애 알림)
//   - DOWN 상태에서 연속 recoveryThreshold회 성공 → UP (복구 알림)
//   - 같은 상태는 반복 알림하지 않는다.
//
// 상태는 JSON 파일에 저장해 프로그램 재시작 후에도 이어간다.
package state

import (
	"encoding/json"
	"os"
	"sync"
	"time"
)

// Status: 서비스 상태
type Status string

const (
	StatusUp   Status = "UP"
	StatusDown Status = "DOWN"
)

// HostState: 서비스 하나의 상태
type HostState struct {
	Status               Status    `json:"status"`
	ConsecutiveFailures  int       `json:"consecutive_failures"`
	ConsecutiveSuccesses int       `json:"consecutive_successes"`
	LastCheck            time.Time `json:"last_check"`
	Since                time.Time `json:"since"` // 현재 상태가 시작된 시각
}

// Store: 전체 서비스 상태 저장소 (동시 접근은 RWMutex로 보호)
type Store struct {
	mu     sync.RWMutex
	path   string
	states map[string]*HostState

	failureThreshold  int
	recoveryThreshold int
}

// New: Store 생성
func New(path string, failureThreshold, recoveryThreshold int) *Store {
	return &Store{
		path:              path,
		states:            make(map[string]*HostState),
		failureThreshold:  failureThreshold,
		recoveryThreshold: recoveryThreshold,
	}
}

// Load: 상태 파일을 읽는다. 파일이 없으면 fresh start로 간주한다 (에러 아님).
func (s *Store) Load() error {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	return json.Unmarshal(data, &s.states)
}

// Save: 상태를 JSON 파일로 저장한다. (임시 파일 + rename으로 원자적 저장)
func (s *Store) Save() error {
	s.mu.RLock()
	data, err := json.MarshalIndent(s.states, "", "  ")
	s.mu.RUnlock()
	if err != nil {
		return err
	}

	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

// RecordResult: 체크 결과 한 건을 반영한다.
// 반환값: (이전 상태, 새 상태, 알림 전송 여부)
//
// 알림 조건:
//   - 어떤 상태든 DOWN이 되는 순간 → 알림 (fresh 시작 시 DOWN 판정 포함)
//   - DOWN → UP 전환 → 알림
//   - fresh 시작 시 UP 판정은 알림 없음 (재시작마다 스팸 방지)
func (s *Store) RecordResult(name string, ok bool, now time.Time) (Status, Status, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	st, exists := s.states[name]
	if !exists {
		st = &HostState{}
		s.states[name] = st
	}
	prev := st.Status
	st.LastCheck = now

	switch {
	case ok:
		st.ConsecutiveFailures = 0
		switch st.Status {
		case StatusDown:
			st.ConsecutiveSuccesses++
			if st.ConsecutiveSuccesses >= s.recoveryThreshold {
				st.Status = StatusUp
				st.ConsecutiveSuccesses = 0
				st.Since = now
			}
		case StatusUp:
			// 이미 UP — 변화 없음
		default: // fresh 시작, 첫 체크가 성공
			st.Status = StatusUp
			st.Since = now
		}

	default: // !ok
		st.ConsecutiveSuccesses = 0
		st.ConsecutiveFailures++
		if st.Status != StatusDown && st.ConsecutiveFailures >= s.failureThreshold {
			st.Status = StatusDown
			st.Since = now
		}
	}

	curr := st.Status
	notify := false
	switch {
	case curr == StatusDown && prev != StatusDown:
		notify = true // UP→DOWN 또는 fresh→DOWN
	case curr == StatusUp && prev == StatusDown:
		notify = true // DOWN→UP 복구
	}
	return prev, curr, notify
}

// Get: 서비스 하나의 상태 (로깅/디버깅용)
func (s *Store) Get(name string) (HostState, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	st, ok := s.states[name]
	if !ok {
		return HostState{}, false
	}
	return *st, true
}

// Snapshot: 전체 상태 복사본 (테스트/디버깅용)
func (s *Store) Snapshot() map[string]HostState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]HostState, len(s.states))
	for k, v := range s.states {
		out[k] = *v
	}
	return out
}
