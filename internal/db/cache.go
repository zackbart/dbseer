package db

import (
	"context"
	"sync"
	"time"
)

// SchemaCache is a thread-safe in-process cache for the introspected Schema
// with a configurable TTL. Use NewSchemaCache to create.
type SchemaCache struct {
	mu      sync.RWMutex
	schema  *Schema
	fetched time.Time
	ttl     time.Duration
}

// NewSchemaCache creates a SchemaCache with the given TTL.
// The plan specifies 30 seconds as the standard TTL.
func NewSchemaCache(ttl time.Duration) *SchemaCache {
	return &SchemaCache{ttl: ttl}
}

// Get returns the cached schema if it is fresh (age < ttl) and refresh is
// false. Otherwise it runs Introspect, updates the cache, and returns the
// fresh schema.
func (c *SchemaCache) Get(ctx context.Context, pool *Pool, refresh bool) (*Schema, error) {
	if !refresh {
		c.mu.RLock()
		if c.schema != nil && time.Since(c.fetched) < c.ttl {
			s := c.schema
			c.mu.RUnlock()
			return s, nil
		}
		c.mu.RUnlock()
	}

	// Need to refetch; take write lock.
	c.mu.Lock()
	defer c.mu.Unlock()

	// Double-check after acquiring write lock.
	if !refresh && c.schema != nil && time.Since(c.fetched) < c.ttl {
		return c.schema, nil
	}

	s, err := Introspect(ctx, pool)
	if err != nil {
		return nil, err
	}
	c.schema = s
	c.fetched = time.Now()
	return s, nil
}

// Invalidate resets the cache so the next call to Get refetches from Postgres.
func (c *SchemaCache) Invalidate() {
	c.mu.Lock()
	c.fetched = time.Time{}
	c.mu.Unlock()
}
