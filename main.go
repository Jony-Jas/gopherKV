package main

import (
	"flag"

	"github.com/jony-jas/gopherKV/config"
	"github.com/jony-jas/gopherKV/server"
)


func setupFlag() {
	flag.StringVar(&config.Host, "host", "0.0.0.0", "Host address for GopherKV")
	flag.IntVar(&config.Port, "port", 7379, "Port for GopherKV")
	flag.Parse()
}


func main() {
	setupFlag()
	server.RunAsyncTCPServer()
}