package tracker

import (
	"context"
	"log"
	"strings"
	"sync"
	"time"

	"polymarket_go/internal/db"
)

var (
	followMu    sync.RWMutex
	followCache map[string][]int64 // wallet → []chatID
)

// StartFollowCache initializes the watchlist cache and refreshes it every 30 seconds.
func StartFollowCache() {
	followCache = make(map[string][]int64)
	refreshFollowCache()
	go func() {
		for {
			time.Sleep(30 * time.Second)
			refreshFollowCache()
		}
	}()
}

func refreshFollowCache() {
	ctx := context.Background()
	m, err := db.GetAllWatchedWallets(ctx)
	if err != nil {
		log.Printf("follow_cache: refresh error: %v", err)
		return
	}
	followMu.Lock()
	followCache = m
	followMu.Unlock()
}

// GetFollowers returns the list of chatIDs watching the given wallet address.
// Wallet addresses are case-insensitive; normalized to lowercase for lookup.
// Safe for concurrent use.
func GetFollowers(wallet string) []int64 {
	followMu.RLock()
	defer followMu.RUnlock()
	return followCache[strings.ToLower(wallet)]
}
