package tracker

import (
	"fmt"
	"log"
	"time"

	"github.com/gorilla/websocket"

	"polymarket_go/internal/metrics"
)

const (
	wsURL    = "wss://ws-live-data.polymarket.com"
	watchDog = 30 * time.Second
)

// StartSocket connects to the Polymarket WebSocket and pushes raw trade messages into jobs.
// It returns an error on disconnect so the caller can reconnect.
func StartSocket(jobs chan<- []byte) error {
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
		ReadBufferSize:   512 * 1024,
	}

	c, _, err := dialer.Dial(wsURL, nil)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	defer c.Close()

	c.SetReadDeadline(time.Now().Add(watchDog))

	sub := map[string]interface{}{
		"action":        "subscribe",
		"subscriptions": []map[string]string{{"topic": "activity", "type": "trades"}},
	}
	if err := c.WriteJSON(sub); err != nil {
		return fmt.Errorf("subscribe: %w", err)
	}

	log.Println("✅ Подписка на Polymarket WS оформлена")

	for {
		_, msgBytes, err := c.ReadMessage()
		if err != nil {
			return fmt.Errorf("read: %w", err)
		}
		c.SetReadDeadline(time.Now().Add(watchDog))
		metrics.WSMessages.Inc()

		// Non-blocking send: drop if buffer full to avoid lag accumulation.
		select {
		case jobs <- msgBytes:
		default:
		}
	}
}
