package core

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestCoreRedisKVStoreUnit(t *testing.T) {
	redisClient := redis.NewClient(&redis.Options{
		Addr: "127.0.0.1:19999",
	})
	redisKVStore := &RedisKVStore{
		universalClient: redisClient,
	}

	// Close
	if err := redisKVStore.Close(); err != nil {
		t.Fatalf("unexpected error on Close: %v", err)
	}

	// Double close returns error since client is already closed
	if err := redisKVStore.Close(); err == nil {
		t.Fatal("expected error on second Close from already closed redis client")
	}

	ctx := context.Background()

	// Empty MGet and empty MSet should succeed even on closed store without reaching network
	emptyMGetResult, err := redisKVStore.MGet(ctx, []string{})
	if err != nil || len(emptyMGetResult) != 0 {
		t.Fatalf("expected empty MGet to succeed without error on closed store, got: %v", err)
	}
	if err := redisKVStore.MSet(ctx, map[string]string{}, time.Minute); err != nil {
		t.Fatalf("expected empty MSet to succeed without error on closed store, got: %v", err)
	}

	if _, err := redisKVStore.Get(ctx, "k"); err == nil {
		t.Fatal("expected error on Get from closed redis store")
	}
	if _, err := redisKVStore.MGet(ctx, []string{"k"}); err == nil {
		t.Fatal("expected error on MGet from closed redis store")
	}
	if err := redisKVStore.Set(ctx, "k", "v", time.Minute); err == nil {
		t.Fatal("expected error on Set from closed redis store")
	}
	if err := redisKVStore.MSet(ctx, map[string]string{"k": "v"}, time.Minute); err == nil {
		t.Fatal("expected error on MSet from closed redis store")
	}
	if _, err := redisKVStore.SetNX(ctx, "k", "v", time.Minute); err == nil {
		t.Fatal("expected error on SetNX from closed redis store")
	}
	if err := redisKVStore.Expire(ctx, "k", time.Minute); err == nil {
		t.Fatal("expected error on Expire from closed redis store")
	}
	if err := redisKVStore.Delete(ctx, "k"); err == nil {
		t.Fatal("expected error on Delete from closed redis store")
	}
	if _, err := redisKVStore.Increment(ctx, "k", time.Minute); err == nil {
		t.Fatal("expected error on Increment from closed redis store")
	}
	if err := redisKVStore.Ping(ctx); err == nil {
		t.Fatal("expected error on Ping from closed redis store")
	}
}
