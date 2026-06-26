package config

var Host string = "0.0.0.0"
var Port int = 7379
var AOFFile string = "./backup.aof"
var KeysLimit int = 5
var EvictionStrategy string = "SIMPLE_FIRST"