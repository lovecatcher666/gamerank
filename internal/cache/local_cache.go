package cache

import (
	"container/list"
	"sync"
	"time"

	"game-leaderboard/internal/model"
)

// CacheItem 缓存项
type CacheItem struct {
	key        string
	value      interface{}
	expiration time.Time
}

// LocalCache 本地缓存实现
type LocalCache struct {
	mu       sync.RWMutex
	items    map[string]*list.Element
	lruList  *list.List
	capacity int
	ttl      time.Duration

	// 统计信息
	hits   int64
	misses int64
}

// NewLocalCache 创建新的本地缓存
func NewLocalCache(capacity int) *LocalCache {
	cache := &LocalCache{
		items:    make(map[string]*list.Element),
		lruList:  list.New(),
		capacity: capacity,
		ttl:      5 * time.Minute, // 默认5分钟过期
	}

	// 启动定期清理
	cache.StartCleanup(1 * time.Minute)

	return cache
}

// SetPlayerRank 缓存玩家排名
func (c *LocalCache) SetPlayerRank(playerID string, rankInfo *model.RankInfo) {
	c.set("rank:"+playerID, rankInfo)
}

// GetPlayerRank 获取缓存的玩家排名
func (c *LocalCache) GetPlayerRank(playerID string) (*model.RankInfo, bool) {
	value, ok := c.get("rank:" + playerID)
	if !ok {
		return nil, false
	}

	if rankInfo, ok := value.(*model.RankInfo); ok {
		return rankInfo, true
	}

	return nil, false
}

// SetTopN 缓存前N名
func (c *LocalCache) SetTopN(n int, rankings []*model.RankInfo) {
	key := "top:" + string(rune(n))
	c.set(key, rankings)
}

// GetTopN 获取缓存的前N名
func (c *LocalCache) GetTopN(n int) ([]*model.RankInfo, bool) {
	key := "top:" + string(rune(n))
	value, ok := c.get(key)
	if !ok {
		return nil, false
	}

	if rankings, ok := value.([]*model.RankInfo); ok {
		return rankings, true
	}

	return nil, false
}

// ClearPlayerRank 清除玩家排名缓存
func (c *LocalCache) ClearPlayerRank(playerID string) {
	c.delete("rank:" + playerID)
}

// ClearTopN 清除前N名缓存
func (c *LocalCache) ClearTopN() {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 清除所有 top: 开头的缓存键
	keysToDelete := make([]string, 0)
	for key := range c.items {
		if len(key) > 4 && key[:4] == "top:" {
			keysToDelete = append(keysToDelete, key)
		}
	}

	for _, key := range keysToDelete {
		c.delete(key)
	}
}

// Clear 清除所有缓存
func (c *LocalCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = make(map[string]*list.Element)
	c.lruList.Init()
	c.hits = 0
	c.misses = 0
}

// GetStats 获取缓存统计信息
func (c *LocalCache) GetStats() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	total := c.hits + c.misses
	hitRate := 0.0
	if total > 0 {
		hitRate = float64(c.hits) / float64(total) * 100
	}

	return map[string]interface{}{
		"hits":     c.hits,
		"misses":   c.misses,
		"hit_rate": hitRate,
		"size":     len(c.items),
		"capacity": c.capacity,
		"usage":    float64(len(c.items)) / float64(c.capacity) * 100,
	}
}

// 内部方法
func (c *LocalCache) set(key string, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 如果键已存在，更新值并移到前面
	if elem, exists := c.items[key]; exists {
		c.lruList.MoveToFront(elem)
		item := elem.Value.(*CacheItem)
		item.value = value
		item.expiration = time.Now().Add(c.ttl)
		return
	}

	// 如果缓存已满，移除最近最少使用的项
	if len(c.items) >= c.capacity {
		c.evict()
	}

	// 创建新缓存项
	item := &CacheItem{
		key:        key,
		value:      value,
		expiration: time.Now().Add(c.ttl),
	}

	// 添加到链表前面并存储引用
	elem := c.lruList.PushFront(item)
	c.items[key] = elem
}

func (c *LocalCache) get(key string) (interface{}, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	elem, exists := c.items[key]
	if !exists {
		c.misses++
		return nil, false
	}

	item := elem.Value.(*CacheItem)

	// 检查是否过期
	if time.Now().After(item.expiration) {
		c.delete(key)
		c.misses++
		return nil, false
	}

	// 移到前面（最近使用）
	c.lruList.MoveToFront(elem)
	c.hits++

	return item.value, true
}

func (c *LocalCache) delete(key string) {
	if elem, exists := c.items[key]; exists {
		c.lruList.Remove(elem)
		delete(c.items, key)
	}
}

func (c *LocalCache) evict() {
	// 从链表尾部移除（最近最少使用）
	elem := c.lruList.Back()
	if elem != nil {
		item := elem.Value.(*CacheItem)
		c.lruList.Remove(elem)
		delete(c.items, item.key)
	}
}

// StartCleanup 启动定期清理过期缓存
func (c *LocalCache) StartCleanup(interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			c.cleanup()
		}
	}()
}

func (c *LocalCache) cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	keysToDelete := make([]string, 0)

	for key, elem := range c.items {
		item := elem.Value.(*CacheItem)
		if now.After(item.expiration) {
			keysToDelete = append(keysToDelete, key)
		}
	}

	for _, key := range keysToDelete {
		c.delete(key)
	}
}
