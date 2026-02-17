package cache

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
)

var RDB *redis.Client

func Init(addr string) {
	RDB = redis.NewClient(&redis.Options{Addr: addr})
}

// GetJSON unmarshals a cached JSON value into dst. Returns false on miss or error.
func GetJSON(ctx context.Context, key string, dst any) bool {
	val, err := RDB.Get(ctx, key).Result()
	if err != nil || val == "" {
		return false
	}
	return json.Unmarshal([]byte(val), dst) == nil
}

// SetJSON marshals val and stores it with the given TTL.
func SetJSON(ctx context.Context, key string, val any, ttl time.Duration) {
	b, err := json.Marshal(val)
	if err != nil {
		return
	}
	RDB.Set(ctx, key, b, ttl)
}

// GetString returns a raw string value from Redis.
func GetString(ctx context.Context, key string) (string, bool) {
	val, err := RDB.Get(ctx, key).Result()
	return val, err == nil && val != ""
}

// SetString stores a plain string.
func SetString(ctx context.Context, key, val string, ttl time.Duration) {
	RDB.Set(ctx, key, val, ttl)
}
