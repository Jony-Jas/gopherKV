package core

import (
	"sort"
)

type PoolItem struct {
	key            string
	lastAccessedAt uint32
}

// TODO: When last accessed at of object changes
// update the poolItem correponding to that
type EvictionPool struct {
	pool   []*PoolItem
	keyset map[string]*PoolItem
}

type ByIdleTime []*PoolItem

func (a ByIdleTime) Len() int {
	return len(a)
}

func (a ByIdleTime) Swap(i, j int) {
	a[i], a[j] = a[j], a[i]
}

func (a ByIdleTime) Less(i, j int) bool {
	return getIdleTime(a[i].lastAccessedAt) > getIdleTime(a[j].lastAccessedAt)
}

// TODO: Make the implementation efficient to not need repeated sorting
func (pq *EvictionPool) Push(key string, lastAccessedAt uint32) {
	if existingItem, ok := pq.keyset[key]; ok {
		// Instantly update the item in the slice without searching for it!
		existingItem.lastAccessedAt = lastAccessedAt 
		
		// Re-sort the pool since the idle time changed
		sort.Sort(ByIdleTime(pq.pool)) 
		return
	}
	
	item := &PoolItem{key: key, lastAccessedAt: lastAccessedAt}
	
	if len(pq.pool) < ePoolSizeMax {
		// Pool is not full, just add it
		pq.keyset[key] = item
		pq.pool = append(pq.pool, item)
		sort.Sort(ByIdleTime(pq.pool))
		
	} else {
		// Pool is full. 
		// pool[0] is the BEST candidate for eviction (highest idle time).
		// pool[len-1] is the WORST candidate for eviction (lowest idle time).
		worstItem := pq.pool[len(pq.pool)-1]
		
		// If the new item has a GREATER idle time than the worst item in the pool,
		// it deserves to take its place.
		if getIdleTime(lastAccessedAt) > getIdleTime(worstItem.lastAccessedAt) {
			// 1. Remove the worst item's key from the map (Fixes memory leak)
			delete(pq.keyset, worstItem.key)
			
			// 2. Replace the worst item with the new item in the slice
			pq.pool[len(pq.pool)-1] = item
			
			// 3. Add the new item to the map
			pq.keyset[key] = item
			
			// 4. Re-sort the pool so the best candidate is back at pool[0]
			sort.Sort(ByIdleTime(pq.pool))
		}
	}
}

func (pq *EvictionPool) Pop() *PoolItem {
	if len(pq.pool) == 0 {
		return nil
	}
	item := pq.pool[0]
	pq.pool = pq.pool[1:]
	delete(pq.keyset, item.key)
	return item
}

func newEvictionPool(size int) *EvictionPool {
	return &EvictionPool{
		pool:   make([]*PoolItem, 0, size), 
		keyset: make(map[string]*PoolItem),
	}
}

var ePoolSizeMax int = 16
var ePool *EvictionPool = newEvictionPool(ePoolSizeMax)