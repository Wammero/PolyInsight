package bot

import (
	"context"
	"log"
	"sync/atomic"
	"time"

	"github.com/mymmrac/telego"

	"polymarket_go/internal/metrics"
)

const (
	// sendQueueCap: 1000 users × 20 buffered alerts comfortably fits without drops.
	sendQueueCap = 20000

	// sendInterval: 40ms between sends = 25 msg/sec, safely under Telegram's 30/sec limit.
	sendInterval = 40 * time.Millisecond
)

var (
	sendCh       chan *telego.SendMessageParams
	sendChClosed atomic.Bool
)

// StartSendQueue starts the single rate-limited Telegram sender goroutine.
// All outgoing messages go through here — guarantees we never exceed Telegram's rate limit.
func StartSendQueue(b *telego.Bot) {
	sendCh = make(chan *telego.SendMessageParams, sendQueueCap)
	go func() {
		ctx := context.Background()
		for params := range sendCh {
			metrics.SendQueueSize.Set(float64(len(sendCh)))

			start := time.Now()
			if _, err := b.SendMessage(ctx, params); err != nil {
				log.Printf("sendq: %v", err)
			}

			// Rate limiting: ensure at least sendInterval between sends.
			// If Telegram API responds faster than 40ms, sleep the remainder.
			// If slower (e.g. 200ms), no sleep needed — we're already under limit.
			if elapsed := time.Since(start); elapsed < sendInterval {
				time.Sleep(sendInterval - elapsed)
			}
		}
	}()
}

// Enqueue adds a message to the rate-limited send queue.
// Non-blocking: if the queue is full (circuit breaker), the message is dropped and counted.
// Safe to call concurrently with DrainAndClose — drops silently if the queue is already closed.
func Enqueue(params *telego.SendMessageParams) {
	if sendChClosed.Load() {
		return
	}
	select {
	case sendCh <- params:
	default:
		metrics.SendQueueDropped.Inc()
	}
}

// DrainAndClose waits up to 30 seconds for the send queue to empty, then closes it.
// Call this during graceful shutdown so in-flight messages are not lost.
func DrainAndClose() {
	deadline := time.Now().Add(30 * time.Second)
	for len(sendCh) > 0 && time.Now().Before(deadline) {
		time.Sleep(100 * time.Millisecond)
	}
	// Mark closed before closing the channel so concurrent Enqueue calls
	// see the flag and return early, preventing "send on closed channel" panics.
	sendChClosed.Store(true)
	close(sendCh)
}
