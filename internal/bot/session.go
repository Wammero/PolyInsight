package bot

import (
	"sync"
	"time"
)

// SessionState tracks what multi-step input we are waiting for from a user.
type SessionState struct {
	State  string    // e.g. "set:amount", "set:trades", "set:odds"
	Expiry time.Time
}

var (
	sessionMu sync.Mutex
	sessions  = make(map[int64]*SessionState)

	// settingsMsgMu guards the settings message ID map.
	// Used to delete the old settings message before sending a new one at the bottom.
	settingsMsgMu sync.Mutex
	settingsMsgs  = make(map[int64]int) // chatID → Telegram MessageID

	// pendingCatsMu guards the pending category selection.
	// Toggles are kept in memory; only written to DB when user clicks "Сохранить".
	pendingCatsMu sync.Mutex
	pendingCats   = make(map[int64][]string) // chatID → pending categories
)

func setSession(chatID int64, state string) {
	sessionMu.Lock()
	defer sessionMu.Unlock()
	sessions[chatID] = &SessionState{State: state, Expiry: time.Now().Add(3 * time.Minute)}
}

func getSession(chatID int64) (string, bool) {
	sessionMu.Lock()
	defer sessionMu.Unlock()
	s, ok := sessions[chatID]
	if !ok {
		return "", false
	}
	if time.Now().After(s.Expiry) {
		delete(sessions, chatID)
		return "", false
	}
	return s.State, true
}

func clearSession(chatID int64) {
	sessionMu.Lock()
	defer sessionMu.Unlock()
	delete(sessions, chatID)
}

func setSettingsMsgID(chatID int64, msgID int) {
	settingsMsgMu.Lock()
	defer settingsMsgMu.Unlock()
	settingsMsgs[chatID] = msgID
}

func getSettingsMsgID(chatID int64) int {
	settingsMsgMu.Lock()
	defer settingsMsgMu.Unlock()
	return settingsMsgs[chatID]
}

func clearSettingsMsgID(chatID int64) {
	settingsMsgMu.Lock()
	defer settingsMsgMu.Unlock()
	delete(settingsMsgs, chatID)
}

// initPendingCats copies the current DB categories into the pending buffer.
func initPendingCats(chatID int64, cats []string) {
	pendingCatsMu.Lock()
	defer pendingCatsMu.Unlock()
	cp := make([]string, len(cats))
	copy(cp, cats)
	pendingCats[chatID] = cp
}

// getPendingCats returns the pending categories, falling back to nil if not set.
func getPendingCats(chatID int64) ([]string, bool) {
	pendingCatsMu.Lock()
	defer pendingCatsMu.Unlock()
	cats, ok := pendingCats[chatID]
	return cats, ok
}

// setPendingCats overwrites the pending categories without touching the DB.
func setPendingCats(chatID int64, cats []string) {
	pendingCatsMu.Lock()
	defer pendingCatsMu.Unlock()
	pendingCats[chatID] = cats
}

// clearPendingCats discards the pending buffer (e.g., on cancel or after save).
func clearPendingCats(chatID int64) {
	pendingCatsMu.Lock()
	defer pendingCatsMu.Unlock()
	delete(pendingCats, chatID)
}
