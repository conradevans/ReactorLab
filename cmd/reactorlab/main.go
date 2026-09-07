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

func main() {
	listen := flag.String("listen", "127.0.0.1:9200", "HTTP listen address")
	frontend := flag.String("frontend", "frontend/dist", "built frontend directory")
	historyPath := flag.String(
		"history",
		"/srv/reactorlab/data/reactorlab.db",
		"ReactorLab history database path",
	)
	flag.Parse()

	host, _, err := net.SplitHostPort(*listen)
	if err != nil {
		log.Fatal(err)
	}

	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		log.Fatal("ReactorLab private listener must use a loopback address")
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

	server := &http.Server{
		Addr:    *listen,
		Handler: api.NewHandler(*frontend),
	}

	serverErrors := make(chan error, 1)
	go func() {
		fmt.Printf("ReactorLab listening on %s\n", *listen)
		serverErrors <- server.ListenAndServe()
	}()

	serverAlreadyStopped := false

	select {
	case <-runContext.Done():
	case err := <-serverErrors:
		serverAlreadyStopped = true
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("ReactorLab HTTP server stopped: %v", err)
		}
		stop()
	}

	shutdownContext, cancelShutdown := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancelShutdown()

	if !serverAlreadyStopped {
		if err := server.Shutdown(shutdownContext); err != nil {
			log.Printf("ReactorLab HTTP shutdown: %v", err)
		}
	}

	stop()
	<-samplerDone

	if !serverAlreadyStopped {
		if err := <-serverErrors; err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("ReactorLab HTTP server stopped: %v", err)
		}
	}
}
