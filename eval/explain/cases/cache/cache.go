package evalcase

import "sync"

type Cache struct {
	mu    sync.Mutex
	items map[string]string
	order []string
	limit int
}

func NewCache(limit int) *Cache {
	return &Cache{items: map[string]string{}, limit: limit}
}

// Put stores v and evicts the oldest inserted key when the cache is full.
func (c *Cache) Put(k, v string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.items[k]; !ok {
		if c.limit > 0 && len(c.order) >= c.limit {
			oldest := c.order[0]
			c.order = c.order[1:]
			delete(c.items, oldest)
		}
		c.order = append(c.order, k)
	}
	c.items[k] = v
}
