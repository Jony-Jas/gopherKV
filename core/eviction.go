package core

import "github.com/jony-jas/gopherKV/config"

// TODO: Make it efficient by doing thorough sampling
func evictFirst() {
	for k := range store {
		delete(store, k)
		return
	}
}

// TODO: Make the eviction strategy configuration driven
// TODO: Support multiple eviction strategies
func evict() {
	switch config.EvictionStrategy {
	case "SIMPLE_FIRST":
		evictFirst()
	}
}