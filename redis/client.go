package redis

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	HashName            = "carplay"
	PublishTimeout      = 1 * time.Second
	HealthCheckInterval = 10 * time.Second
	ReconnectMinBackoff = 1 * time.Second
	ReconnectMaxBackoff = 30 * time.Second
)

type Client struct {
	rdb  *redis.Client
	ctx  context.Context
	addr string // Store address for reconnection

	// Connection state tracking
	mu        sync.RWMutex
	connected bool

	// Health check management
	stopChan chan struct{}
	doneChan chan struct{}
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

		// Retry configuration for transient failures
		MaxRetries:      3,
		MinRetryBackoff: 100 * time.Millisecond,
		MaxRetryBackoff: 2 * time.Second,

		// Connection pool configuration
		PoolSize:     10,
		MinIdleConns: 2,
		PoolTimeout:  4 * time.Second,
	})

	client := &Client{
		rdb:       rdb,
		ctx:       context.Background(),
		addr:      addr,
		connected: false,
		stopChan:  make(chan struct{}),
		doneChan:  make(chan struct{}),
	}

	// Perform initial connection check
	if err := client.Ping(); err == nil {
		client.mu.Lock()
		client.connected = true
		client.mu.Unlock()
		log.Println("[Redis] Initial connection successful")
	} else {
		log.Printf("[Redis] Initial connection failed: %v (will retry in background)", err)
	}

	// Start background health check
	client.startHealthCheck()

	return client
}

// startHealthCheck starts a background goroutine to monitor Redis connectivity
func (c *Client) startHealthCheck() {
	go func() {
		defer close(c.doneChan)

		ticker := time.NewTicker(HealthCheckInterval)
		defer ticker.Stop()

		log.Println("[Redis] Health check started")

		for {
			select {
			case <-c.stopChan:
				log.Println("[Redis] Health check stopped")
				return
			case <-ticker.C:
				// Ping Redis to check connectivity
				err := c.Ping()

				c.mu.Lock()
				wasConnected := c.connected
				c.mu.Unlock()

				if err != nil {
					if wasConnected {
						log.Printf("[Redis] Connection lost: %v", err)
						c.mu.Lock()
						c.connected = false
						c.mu.Unlock()
					}

					// Attempt reconnection
					log.Println("[Redis] Attempting reconnection...")
					if c.reconnect() {
						c.mu.Lock()
						c.connected = true
						c.mu.Unlock()
						log.Println("[Redis] Reconnected successfully")
					} else {
						log.Println("[Redis] Reconnection failed, will retry later")
					}
				} else {
					// Successfully pinged
					if !wasConnected {
						log.Println("[Redis] Connection restored")
						c.mu.Lock()
						c.connected = true
						c.mu.Unlock()
					}
				}
			}
		}
	}()
}

// reconnect attempts to reconnect to Redis with exponential backoff
// Retries indefinitely until connection is restored
func (c *Client) reconnect() bool {
	backoff := ReconnectMinBackoff

	for attempt := 1; ; attempt++ {
		log.Printf("[Redis] Reconnection attempt %d...", attempt)

		// Try to ping
		err := c.Ping()
		if err == nil {
			return true
		}

		log.Printf("[Redis] Attempt %d failed: %v", attempt, err)

		// Wait before next attempt
		time.Sleep(backoff)

		// Exponential backoff with max limit
		backoff *= 2
		if backoff > ReconnectMaxBackoff {
			backoff = ReconnectMaxBackoff
		}
	}
}

// isConnected returns the current connection state (thread-safe)
func (c *Client) isConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

// PublishState sets a key-value pair in the carplay hash and publishes the change
// Pattern: HSET carplay <key> <value> followed by PUBLISH carplay <key>
func (c *Client) PublishState(key, value string) {
	if c == nil || c.rdb == nil {
		return
	}

	// Check connection state
	if !c.isConnected() {
		log.Printf("[Redis] Not connected, skipping publish: %s.%s = %s", HashName, key, value)
		return
	}

	ctx, cancel := context.WithTimeout(c.ctx, PublishTimeout)
	defer cancel()

	// HSET carplay <key> <value>
	err := c.rdb.HSet(ctx, HashName, key, value).Err()
	if err != nil {
		log.Printf("[Redis] Failed to HSET %s %s=%s: %v", HashName, key, value, err)
		// Mark as disconnected so health check will attempt reconnection
		c.mu.Lock()
		c.connected = false
		c.mu.Unlock()
		return
	}

	// PUBLISH carplay <key>
	err = c.rdb.Publish(ctx, HashName, key).Err()
	if err != nil {
		log.Printf("[Redis] Failed to PUBLISH %s %s: %v", HashName, key, err)
		// Mark as disconnected so health check will attempt reconnection
		c.mu.Lock()
		c.connected = false
		c.mu.Unlock()
		return
	}

	log.Printf("[Redis] Published: %s.%s = %s", HashName, key, value)
}

// Close closes the Redis connection and stops the health check
func (c *Client) Close() error {
	if c == nil {
		return nil
	}

	log.Println("[Redis] Closing connection...")

	// Stop health check goroutine
	if c.stopChan != nil {
		close(c.stopChan)
		// Wait for health check to finish
		<-c.doneChan
	}

	// Close Redis connection
	if c.rdb != nil {
		if err := c.rdb.Close(); err != nil {
			log.Printf("[Redis] Error closing connection: %v", err)
			return err
		}
	}

	c.mu.Lock()
	c.connected = false
	c.mu.Unlock()

	log.Println("[Redis] Connection closed")
	return nil
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
