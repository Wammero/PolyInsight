package bot

import (
	"testing"
	"time"
)

func TestSession_SetGet(t *testing.T) {
	chatID := int64(12345)
	setSession(chatID, "set:amount")

	state, ok := getSession(chatID)
	if !ok {
		t.Fatal("expected session to exist")
	}
	if state != "set:amount" {
		t.Errorf("expected state 'set:amount', got %q", state)
	}

	// Cleanup
	clearSession(chatID)
	_, ok = getSession(chatID)
	if ok {
		t.Error("session should be cleared")
	}
}

func TestSession_Expiry(t *testing.T) {
	chatID := int64(99999)

	// Manually create an expired session.
	sessionMu.Lock()
	sessions[chatID] = &SessionState{
		State:  "set:trades",
		Expiry: time.Now().Add(-1 * time.Second),
	}
	sessionMu.Unlock()

	_, ok := getSession(chatID)
	if ok {
		t.Error("expired session should not be returned")
	}

	// Verify it was cleaned up from the map.
	sessionMu.Lock()
	_, exists := sessions[chatID]
	sessionMu.Unlock()
	if exists {
		t.Error("expired session should be deleted from map")
	}
}

func TestSession_Overwrite(t *testing.T) {
	chatID := int64(11111)
	setSession(chatID, "set:amount")
	setSession(chatID, "set:odds")

	state, ok := getSession(chatID)
	if !ok || state != "set:odds" {
		t.Errorf("expected 'set:odds', got %q (ok=%v)", state, ok)
	}
	clearSession(chatID)
}

func TestSettingsMsgID_SetGetClear(t *testing.T) {
	chatID := int64(22222)
	setSettingsMsgID(chatID, 42)

	got := getSettingsMsgID(chatID)
	if got != 42 {
		t.Errorf("expected msgID 42, got %d", got)
	}

	clearSettingsMsgID(chatID)
	got = getSettingsMsgID(chatID)
	if got != 0 {
		t.Errorf("expected 0 after clear, got %d", got)
	}
}

func TestSettingsMsgID_Expired(t *testing.T) {
	chatID := int64(33333)

	settingsMsgMu.Lock()
	settingsMsgs[chatID] = settingsMsgEntry{
		MsgID:  100,
		Expiry: time.Now().Add(-1 * time.Second),
	}
	settingsMsgMu.Unlock()

	got := getSettingsMsgID(chatID)
	if got != 0 {
		t.Errorf("expired settingsMsgID should return 0, got %d", got)
	}
}

func TestPendingCats_SetGetClear(t *testing.T) {
	chatID := int64(44444)
	initPendingCats(chatID, []string{"politics", "crypto"})

	cats, ok := getPendingCats(chatID)
	if !ok || len(cats) != 2 {
		t.Fatalf("expected 2 categories, got %v (ok=%v)", cats, ok)
	}
	if cats[0] != "politics" || cats[1] != "crypto" {
		t.Errorf("expected [politics, crypto], got %v", cats)
	}

	// Overwrite
	setPendingCats(chatID, []string{"tech"})
	cats, ok = getPendingCats(chatID)
	if !ok || len(cats) != 1 || cats[0] != "tech" {
		t.Errorf("expected [tech], got %v", cats)
	}

	clearPendingCats(chatID)
	_, ok = getPendingCats(chatID)
	if ok {
		t.Error("pending cats should be cleared")
	}
}

func TestPendingCats_Expired(t *testing.T) {
	chatID := int64(55555)

	pendingCatsMu.Lock()
	pendingCats[chatID] = pendingCatsEntry{
		Cats:   []string{"finance"},
		Expiry: time.Now().Add(-1 * time.Second),
	}
	pendingCatsMu.Unlock()

	_, ok := getPendingCats(chatID)
	if ok {
		t.Error("expired pending cats should not be returned")
	}
}

func TestPendingCats_IsolatedCopy(t *testing.T) {
	chatID := int64(66666)
	original := []string{"politics", "crypto"}
	initPendingCats(chatID, original)

	// Modify the original slice — should not affect the stored copy.
	original[0] = "MODIFIED"

	cats, ok := getPendingCats(chatID)
	if !ok {
		t.Fatal("expected pending cats")
	}
	if cats[0] == "MODIFIED" {
		t.Error("initPendingCats should make a copy, not reference the original slice")
	}

	clearPendingCats(chatID)
}

func TestConcurrentSessionAccess(t *testing.T) {
	// Test that concurrent access doesn't panic (race detector check).
	done := make(chan struct{})
	for i := 0; i < 100; i++ {
		go func(id int64) {
			defer func() { done <- struct{}{} }()
			setSession(id, "set:amount")
			getSession(id)
			clearSession(id)
		}(int64(i))
	}
	for i := 0; i < 100; i++ {
		<-done
	}
}
