package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/conradevans/ReactorLab/internal/api"
	"github.com/conradevans/ReactorLab/internal/history"
	"github.com/conradevans/ReactorLab/internal/metrics"
)

type serverResult struct {
	name string
	err  error
}

func main() {
	listen := flag.String("listen", "127.0.0.1:9200", "private HTTP listen address")
	publicListen := flag.String(
		"public-listen",
		"127.0.0.1:9203",
		"public guest HTTP listen address",
	)
	frontend := flag.String("frontend", "frontend/dist", "built frontend directory")
	historyPath := flag.String(
		"history",
		"/srv/reactorlab/data/reactorlab.db",
		"ReactorLab history database path",
	)
	flag.Parse()

	requireLoopback("private", *listen)
	requireLoopback("public", *publicListen)
	if *listen == *publicListen {
		log.Fatal("ReactorLab private and public listeners must be different")
	}

	historyStore, err := history.Open(*historyPath)
	if err != nil {
		log.Fatalf("open ReactorLab history: %v", err)
	}
	defer func() {
		if err := historyStore.Close(); err != nil {
			log.Printf("close ReactorLab history: %v", err)
		}
	}()

	runContext, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	sampler := metrics.NewSampler(historyStore)
	samplerDone := make(chan struct{})
	go func() {
		defer close(samplerDone)
		sampler.Run(runContext)
	}()

	servers := []struct {
		name   string
		server *http.Server
	}{
		{
			name: "private",
			server: &http.Server{
				Addr:    *listen,
				Handler: api.NewHandlerWithHistory(*frontend, historyStore),
			},
		},
		{
			name: "public",
			server: &http.Server{
				Addr:    *publicListen,
				Handler: api.NewPublicHandler(*frontend),
			},
		},
	}

	serverResults := make(chan serverResult, len(servers))
	for _, item := range servers {
		item := item
		go func() {
			fmt.Printf("ReactorLab %s listener on %s\n", item.name, item.server.Addr)
			serverResults <- serverResult{
				name: item.name,
				err:  item.server.ListenAndServe(),
			}
		}()
	}

	resultsSeen := 0
	select {
	case <-runContext.Done():
	case result := <-serverResults:
		resultsSeen = 1
		logServerResult(result)
		stop()
	}

	shutdownContext, cancelShutdown := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancelShutdown()

	for _, item := range servers {
		if err := item.server.Shutdown(shutdownContext); err != nil {
			log.Printf("ReactorLab %s HTTP shutdown: %v", item.name, err)
		}
	}

	stop()
	<-samplerDone

	for resultsSeen < len(servers) {
		logServerResult(<-serverResults)
		resultsSeen++
	}
}

func requireLoopback(name, address string) {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		log.Fatalf("invalid ReactorLab %s listener: %v", name, err)
	}

	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		log.Fatalf("ReactorLab %s listener must use a loopback address", name)
	}
}

func logServerResult(result serverResult) {
	if result.err != nil && !errors.Is(result.err, http.ErrServerClosed) {
		log.Printf("ReactorLab %s HTTP server stopped: %v", result.name, result.err)
	}
}
