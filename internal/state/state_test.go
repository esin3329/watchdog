package state

import (
	"path/filepath"
	"testing"
	"time"
)

func mustStore(t *testing.T, path string) *Store {
	t.Helper()
	return New(path, 3, 2) // failure 3회, recovery 2회
}

func TestUpToDownAfterThreeFailures(t *testing.T) {
	s := mustStore(t, filepath.Join(t.TempDir(), "state.json"))
	now := time.Now()

	// 첫 체크 성공 → UP (fresh, 알림 없음)
	prev, curr, notify := s.RecordResult("svc", true, now)
	if prev != "" || curr != StatusUp || notify {
		t.Fatalf("fresh UP: got prev=%q curr=%q notify=%v, want prev=\"\" curr=UP notify=false", prev, curr, notify)
	}

	// 실패 1, 2회 → 여전히 UP, 알림 없음
	for i := 1; i <= 2; i++ {
		prev, curr, notify = s.RecordResult("svc", false, now)
		if prev != StatusUp || curr != StatusUp || notify {
			t.Fatalf("실패 %d회: got prev=%q curr=%q notify=%v", i, prev, curr, notify)
		}
	}

	// 실패 3회 → DOWN + 알림
	prev, curr, notify = s.RecordResult("svc", false, now)
	if prev != StatusUp || curr != StatusDown || !notify {
		t.Fatalf("실패 3회: got prev=%q curr=%q notify=%v, want prev=UP curr=DOWN notify=true", prev, curr, notify)
	}
}

func TestDownToUpAfterRecoveryThreshold(t *testing.T) {
	s := mustStore(t, filepath.Join(t.TempDir(), "state.json"))
	now := time.Now()

	// DOWN 상태로 만들기
	s.RecordResult("svc", true, now)
	s.RecordResult("svc", false, now)
	s.RecordResult("svc", false, now)
	s.RecordResult("svc", false, now)
	if st, _ := s.Get("svc"); st.Status != StatusDown {
		t.Fatalf("DOWN 준비 실패: %s", st.Status)
	}

	// 복구 1회 → 아직 DOWN, 알림 없음 (히스테리시스)
	prev, curr, notify := s.RecordResult("svc", true, now)
	if prev != StatusDown || curr != StatusDown || notify {
		t.Fatalf("복구 1회: got prev=%q curr=%q notify=%v, want DOWN 유지", prev, curr, notify)
	}

	// 복구 2회 → UP + 알림
	prev, curr, notify = s.RecordResult("svc", true, now)
	if prev != StatusDown || curr != StatusUp || !notify {
		t.Fatalf("복구 2회: got prev=%q curr=%q notify=%v, want prev=DOWN curr=UP notify=true", prev, curr, notify)
	}
}

func TestNoRepeatNotificationOnSameState(t *testing.T) {
	s := mustStore(t, filepath.Join(t.TempDir(), "state.json"))
	now := time.Now()

	// DOWN 만들기
	s.RecordResult("svc", true, now)
	s.RecordResult("svc", false, now)
	s.RecordResult("svc", false, now)
	s.RecordResult("svc", false, now)

	// DOWN 상태 추가 실패 → 알림 없음 (반복 전송 금지)
	for i := 0; i < 3; i++ {
		prev, curr, notify := s.RecordResult("svc", false, now)
		if notify {
			t.Fatalf("DOWN 반복 실패에서 알림 발생: prev=%q curr=%q", prev, curr)
		}
	}
}

func TestFreshStartDownStillNotifies(t *testing.T) {
	s := mustStore(t, filepath.Join(t.TempDir(), "state.json"))
	now := time.Now()

	// fresh 상태에서 연속 3회 실패 → DOWN + 알림
	s.RecordResult("svc", false, now)
	s.RecordResult("svc", false, now)
	prev, curr, notify := s.RecordResult("svc", false, now)

	if prev != "" || curr != StatusDown || !notify {
		t.Fatalf("fresh DOWN: got prev=%q curr=%q notify=%v", prev, curr, notify)
	}
}

func TestSaveAndLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	s := mustStore(t, path)
	now := time.Now()

	s.RecordResult("svc", true, now)
	s.RecordResult("svc", false, now)
	s.RecordResult("svc", false, now)
	s.RecordResult("svc", false, now) // DOWN

	if err := s.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// 새 Store로 로드 → 상태 복원
	s2 := New(path, 3, 2)
	if err := s2.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}

	st, ok := s2.Get("svc")
	if !ok || st.Status != StatusDown {
		t.Fatalf("상태 복원 실패: ok=%v status=%q", ok, st.Status)
	}

	// 복원된 DOWN 상태에서 복구 알림이 동작해야 한다
	s2.RecordResult("svc", true, now)                          // 1회: 아직 DOWN
	prev2, curr2, notify2 := s2.RecordResult("svc", true, now) // 2회: UP + 알림
	if prev2 != StatusDown || curr2 != StatusUp || !notify2 {
		t.Fatalf("복원 후 복구 판정 실패: prev=%q curr=%q notify=%v", prev2, curr2, notify2)
	}
}

func TestLoadMissingFileIsFreshStart(t *testing.T) {
	s := mustStore(t, filepath.Join(t.TempDir(), "no-such-file.json"))

	if err := s.Load(); err != nil {
		t.Fatalf("파일 없는 Load는 nil이어야 함: %v", err)
	}
	if got := len(s.Snapshot()); got != 0 {
		t.Fatalf("fresh 상태여야 함: %d개", got)
	}
}

func TestSameNameUpdate(t *testing.T) {
	s := mustStore(t, filepath.Join(t.TempDir(), "state.json"))
	now := time.Now()

	// UP 유지 중 성공 → 상태 유지
	s.RecordResult("svc", true, now)
	prev, curr, notify := s.RecordResult("svc", true, now)

	if prev != StatusUp || curr != StatusUp || notify {
		t.Fatalf("UP 유지: got prev=%q curr=%q notify=%v", prev, curr, notify)
	}
}
