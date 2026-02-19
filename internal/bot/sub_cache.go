package bot

import (
	"context"
	"log"
	"sync"
	"time"

	"polymarket_go/internal/db"
	"polymarket_go/internal/metrics"
)

var (
	subMu    sync.RWMutex
	subCache []db.Subscriber
)

// StartSubCache keeps the full subscriber list in memory, refreshed every 5 seconds.
// Eliminates one AllSubscribers() DB query per whale event — critical at 1000+ users.
func StartSubCache() {
	ctx := context.Background()
	refresh := func() {
		subs, err := db.AllSubscribers(ctx)
		if err != nil {
			log.Printf("sub_cache: refresh error: %v", err)
			return
		}
		subMu.Lock()
		subCache = subs
		subMu.Unlock()
		metrics.AllUsers.Set(float64(len(subs)))
	}
	refresh() // initial load before returning so first alerts are ready
	go func() {
		for {
			time.Sleep(5 * time.Second)
			refresh()
		}
	}()
}

// GetSubscribers returns the cached subscriber list. O(1), no DB call.
func GetSubscribers() []db.Subscriber {
	subMu.RLock()
	defer subMu.RUnlock()
	return subCache
}
