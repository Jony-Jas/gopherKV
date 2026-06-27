package main

import (
	"flag"
	"os"
	"os/signal"
	"sync"
	"syscall"

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
	var sigs chan os.Signal = make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGTERM, syscall.SIGINT)
	var wg sync.WaitGroup
	wg.Add(2)

	go server.RunAsyncTCPServer(&wg)
	go server.WaitForSignal(&wg, sigs)

	wg.Wait()
}