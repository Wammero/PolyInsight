package shortener

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"polymarket_go/internal/cache"
)

var client = &http.Client{Timeout: 3 * time.Second}

// Shorten returns a tracked is.gd short URL for the given long URL.
// Results are cached in Redis for 30 days to avoid redundant API calls
// for repeated links (e.g., same wallet profile across multiple alerts).
// Falls back to the original URL on any error so the bot never breaks.
func Shorten(ctx context.Context, longURL string) string {
	cacheKey := "isgd:" + longURL
	if short, ok := cache.GetString(ctx, cacheKey); ok {
		return short
	}

	apiURL := fmt.Sprintf(
		"https://is.gd/create.php?format=json&logstats=1&url=%s",
		url.QueryEscape(longURL),
	)
	resp, err := client.Get(apiURL)
	if err != nil || resp == nil {
		return longURL
	}
	defer func() { io.Copy(io.Discard, resp.Body); resp.Body.Close() }()

	var result struct {
		ShortURL string `json:"shorturl"`
		ErrorCode int   `json:"errorcode"`
	}
	if json.NewDecoder(resp.Body).Decode(&result) != nil || result.ShortURL == "" {
		return longURL
	}

	cache.SetString(ctx, cacheKey, result.ShortURL, 30*24*time.Hour)
	return result.ShortURL
}
