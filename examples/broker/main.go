package main

import (
	"log"
	"net"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/zwczou/mqtt/broker"
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	// see godoc net/http/pprof
	go func() {
		log.Println(http.ListenAndServe("0.0.0.0:6060", nil))
	}()

	l, err := net.Listen("tcp", ":1883")
	if err != nil {
		log.Printf("ERROR: failed to listen - %s", err)
		return
	}

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	svr := broker.NewServer(l)
	svr.Start()
	<-signalChan
	svr.Stop()
}
