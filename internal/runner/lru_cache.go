package runner

import (
	"container/list"
	"sync"
)

// lruCache 是一个固定容量的 LRU 缓存，键值均为 string。
// 超过容量时淘汰最久未访问的条目。线程安全。
// 用于 transpileCache，避免长会话下不同历史版本源码缓存无限增长导致内存膨胀。
type lruCache struct {
	mu       sync.Mutex
	cap      int
	m        map[string]*list.Element
	order    *list.List // 前端为最近使用，尾端为最久未使用
}

type lruEntry struct {
	key, value string
}

// newLRUCache 创建容量为 cap 的 LRU 缓存。cap <= 0 时按 1 处理。
func newLRUCache(cap int) *lruCache {
	if cap <= 0 {
		cap = 1
	}
	return &lruCache{
		cap:   cap,
		m:     make(map[string]*list.Element, cap),
		order: list.New(),
	}
}

// get 查询 key，存在时返回 value 并将其移到最近使用端。
func (c *lruCache) get(key string) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.m[key]; ok {
		c.order.MoveToFront(el)
		return el.Value.(*lruEntry).value, true
	}
	return "", false
}

// set 写入 key-value。已存在则更新并移到最近使用端；不存在则插入，
// 超过容量时淘汰尾端（最久未使用）条目。
func (c *lruCache) set(key, value string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.m[key]; ok {
		el.Value.(*lruEntry).value = value
		c.order.MoveToFront(el)
		return
	}
	el := c.order.PushFront(&lruEntry{key: key, value: value})
	c.m[key] = el
	if c.order.Len() > c.cap {
		// 淘汰尾端
		tail := c.order.Back()
		if tail != nil {
			c.order.Remove(tail)
			delete(c.m, tail.Value.(*lruEntry).key)
		}
	}
}

// len 返回当前条目数（调试/监控用）。
func (c *lruCache) len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.order.Len()
}
