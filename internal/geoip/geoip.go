package geoip

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"time"

	"polymarket_go/internal/cache"
)

var client = &http.Client{Timeout: 3 * time.Second}

// Country returns the ISO country code (e.g. "US", "RU") for the given IP.
// Results are cached in Redis for 30 days so each unique IP is looked up once.
// Returns "unknown" on any error or if the IP is local/invalid.
func Country(ctx context.Context, ip string) string {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return "unknown"
	}
	if parsed.IsLoopback() || parsed.IsPrivate() {
		return "local"
	}

	cacheKey := "geoip:" + ip
	if country, ok := cache.GetString(ctx, cacheKey); ok {
		return country
	}

	// ip-api.com: free, no key required, 45 req/min.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://ip-api.com/json/"+ip+"?fields=countryCode", nil)
	if err != nil {
		return "unknown"
	}
	resp, err := client.Do(req)
	if err != nil || resp == nil {
		return "unknown"
	}
	defer func() { io.Copy(io.Discard, resp.Body); resp.Body.Close() }()

	var result struct {
		CountryCode string `json:"countryCode"`
	}
	if json.NewDecoder(resp.Body).Decode(&result) != nil || result.CountryCode == "" {
		return "unknown"
	}

	cache.SetString(ctx, cacheKey, result.CountryCode, 30*24*time.Hour)
	return result.CountryCode
}
