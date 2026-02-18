package bot

import (
	"sync"
	"time"
)

// SessionState tracks what multi-step input we are waiting for from a user.
type SessionState struct {
	State  string
	Expiry time.Time
}

// settingsMsgEntry holds a Telegram message ID with an expiry for automatic cleanup.
type settingsMsgEntry struct {
	MsgID  int
	Expiry time.Time
}

// pendingCatsEntry holds pending category selection with an expiry for automatic cleanup.
type pendingCatsEntry struct {
	Cats   []string
	Expiry time.Time
}

const (
	sessionTTL     = 3 * time.Minute
	settingsMsgTTL = 30 * time.Minute
	pendingCatsTTL = 10 * time.Minute
	cleanupPeriod  = 5 * time.Minute
)

var (
	sessionMu sync.Mutex
	sessions  = make(map[int64]*SessionState)

	// settingsMsgMu guards the settings message ID map.
	// Used to delete the old settings message before sending a new one.
	settingsMsgMu sync.Mutex
	settingsMsgs  = make(map[int64]settingsMsgEntry)

	// pendingCatsMu guards the pending category selection.
	// Toggles are kept in memory; only written to DB when user clicks "Сохранить".
	pendingCatsMu sync.Mutex
	pendingCats   = make(map[int64]pendingCatsEntry)
)

// StartSessionCleanup starts a background goroutine that periodically evicts
// expired entries from all three in-memory maps, preventing unbounded growth.
func StartSessionCleanup() {
	go func() {
		for {
			time.Sleep(cleanupPeriod)
			now := time.Now()

			sessionMu.Lock()
			for id, s := range sessions {
				if now.After(s.Expiry) {
					delete(sessions, id)
				}
			}
			sessionMu.Unlock()

			settingsMsgMu.Lock()
			for id, e := range settingsMsgs {
				if now.After(e.Expiry) {
					delete(settingsMsgs, id)
				}
			}
			settingsMsgMu.Unlock()

			pendingCatsMu.Lock()
			for id, e := range pendingCats {
				if now.After(e.Expiry) {
					delete(pendingCats, id)
				}
			}
			pendingCatsMu.Unlock()
		}
	}()
}

func setSession(chatID int64, state string) {
	sessionMu.Lock()
	defer sessionMu.Unlock()
	sessions[chatID] = &SessionState{State: state, Expiry: time.Now().Add(sessionTTL)}
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
	settingsMsgs[chatID] = settingsMsgEntry{MsgID: msgID, Expiry: time.Now().Add(settingsMsgTTL)}
}

func getSettingsMsgID(chatID int64) int {
	settingsMsgMu.Lock()
	defer settingsMsgMu.Unlock()
	e, ok := settingsMsgs[chatID]
	if !ok || time.Now().After(e.Expiry) {
		delete(settingsMsgs, chatID)
		return 0
	}
	return e.MsgID
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
	pendingCats[chatID] = pendingCatsEntry{Cats: cp, Expiry: time.Now().Add(pendingCatsTTL)}
}

// getPendingCats returns the pending categories, falling back to nil if not set or expired.
func getPendingCats(chatID int64) ([]string, bool) {
	pendingCatsMu.Lock()
	defer pendingCatsMu.Unlock()
	e, ok := pendingCats[chatID]
	if !ok || time.Now().After(e.Expiry) {
		delete(pendingCats, chatID)
		return nil, false
	}
	return e.Cats, true
}

// setPendingCats overwrites the pending categories without touching the DB.
func setPendingCats(chatID int64, cats []string) {
	pendingCatsMu.Lock()
	defer pendingCatsMu.Unlock()
	pendingCats[chatID] = pendingCatsEntry{Cats: cats, Expiry: time.Now().Add(pendingCatsTTL)}
}

// clearPendingCats discards the pending buffer (e.g., on cancel or after save).
func clearPendingCats(chatID int64) {
	pendingCatsMu.Lock()
	defer pendingCatsMu.Unlock()
	delete(pendingCats, chatID)
}
