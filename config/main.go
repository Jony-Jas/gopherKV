package config

var Host string = "0.0.0.0"
var Port int = 7379
var AOFFile string = "./backup.aof"
var KeysLimit int = 100

// var EvictionStrategy string = "SIMPLE_FIRST"
// var EvictionStrategy string = "ALL_KEYS_RANDOM"
var EvictionStrategy string = "ALL_KEYS_LRU"
var EvictionRatio float64 = 0.40