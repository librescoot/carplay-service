package redis

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	HashName       = "carplay"
	PublishTimeout = 1 * time.Second
)

type Client struct {
	rdb *redis.Client
	ctx context.Context
}

// NewClient creates a new Redis client for the carplay service
func NewClient(addr string) *Client {
	rdb := redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     "",
		DB:           0,
		DialTimeout:  2 * time.Second,
		ReadTimeout:  1 * time.Second,
		WriteTimeout: 1 * time.Second,
	})

	return &Client{
		rdb: rdb,
		ctx: context.Background(),
	}
}

// PublishState sets a key-value pair in the carplay hash and publishes the change
// Pattern: HSET carplay <key> <value> followed by PUBLISH carplay <key>
func (c *Client) PublishState(key, value string) {
	if c == nil || c.rdb == nil {
		return
	}

	ctx, cancel := context.WithTimeout(c.ctx, PublishTimeout)
	defer cancel()

	// HSET carplay <key> <value>
	err := c.rdb.HSet(ctx, HashName, key, value).Err()
	if err != nil {
		log.Printf("[Redis] Failed to HSET %s %s=%s: %v", HashName, key, value, err)
		return
	}

	// PUBLISH carplay <key>
	err = c.rdb.Publish(ctx, HashName, key).Err()
	if err != nil {
		log.Printf("[Redis] Failed to PUBLISH %s %s: %v", HashName, key, err)
		return
	}

	log.Printf("[Redis] Published: %s.%s = %s", HashName, key, value)
}

// Close closes the Redis connection
func (c *Client) Close() error {
	if c == nil || c.rdb == nil {
		return nil
	}
	return c.rdb.Close()
}

// Ping checks if Redis is reachable
func (c *Client) Ping() error {
	if c == nil || c.rdb == nil {
		return fmt.Errorf("redis client not initialized")
	}

	ctx, cancel := context.WithTimeout(c.ctx, 2*time.Second)
	defer cancel()

	return c.rdb.Ping(ctx).Err()
}
