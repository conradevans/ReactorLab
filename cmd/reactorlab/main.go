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

	"github.com/conradevans/ReactorLab/internal/accessauth"
	"github.com/conradevans/ReactorLab/internal/api"
	"github.com/conradevans/ReactorLab/internal/history"
	"github.com/conradevans/ReactorLab/internal/metrics"
	"github.com/conradevans/ReactorLab/internal/observability"
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
	observabilityPath := flag.String(
		"observability",
		"/srv/reactorlab/data/observability.db",
		"ReactorLab historical observability database path",
	)
	miniDeployURL := flag.String("minideploy-url", "http://127.0.0.1:9000", "MiniDeploy management base URL")
	miniBaseURL := flag.String("minibase-url", "http://127.0.0.1:9100", "MiniBase management base URL")
	miniAIURL := flag.String("miniai-url", "http://127.0.0.1:9300", "MiniAI base URL")
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

	var (
		observabilityStore  *observability.Store
		observabilityQuery  *observability.QueryService
		observabilityDone   chan struct{}
		recoveryDiscordDone chan struct{}
	)
	observabilityStore, err = observability.Open(*observabilityPath)
	if err != nil {
		log.Printf("warning: historical observability unavailable: %v", err)
		observabilityStore = nil
	} else {
		observabilityQuery = observability.NewQueryService(observabilityStore)
		recoveryDiscordConfig, recoveryDiscordConfigErr :=
			observability.RecoveryDiscordConfigFromEnvironment()
		if recoveryDiscordConfigErr != nil && !recoveryDiscordConfig.Enabled {
			log.Print("warning: invalid recovery Discord configuration; notification creation disabled")
		}
		if recoveryDiscordConfig.TimezoneFallback {
			log.Print("warning: invalid recovery notification timezone; using UTC")
		}
		collector := observability.NewCollector(observabilityStore, observability.CollectorConfig{
			MiniDeployURL:          *miniDeployURL,
			MiniBaseURL:            *miniBaseURL,
			MiniAIURL:              *miniAIURL,
			RecoveryDiscordEnabled: recoveryDiscordConfig.Enabled,
		})
		observabilityDone = make(chan struct{})
		go func() {
			defer close(observabilityDone)
			collector.Run(runContext)
		}()

		if recoveryDiscordConfig.Enabled {
			var recoveryDiscordSender observability.RecoveryDiscordSender
			if recoveryDiscordConfigErr == nil {
				sender, senderErr := observability.NewDiscordWebhookSender(recoveryDiscordConfig)
				if senderErr == nil {
					recoveryDiscordSender = sender
				} else {
					recoveryDiscordConfigErr = senderErr
				}
			}
			if recoveryDiscordConfigErr != nil {
				log.Print("warning: recovery Discord delivery configuration unavailable; queued notifications will retry")
			}
			worker := observability.NewRecoveryDiscordWorker(
				observabilityStore,
				recoveryDiscordSender,
				recoveryDiscordConfig.Location,
			)
			recoveryDiscordDone = make(chan struct{})
			go func() {
				defer close(recoveryDiscordDone)
				worker.Run(runContext)
			}()
		}
	}
	accessValidator, err := accessauth.NewCloudflareValidator(
		accessauth.ConfigFromEnvironment(),
	)
	if err != nil {
		log.Printf(
			"warning: ReactorLab Access session identity unavailable: %v",
			err,
		)
		accessValidator = nil
	}

	privateHandler := api.NewHandlerWithHistoryAccessAndObservability(
		*frontend,
		historyStore,
		accessValidator,
		nil,
		*miniDeployURL,
		*miniBaseURL,
	)
	if observabilityQuery != nil {
		privateHandler = api.NewHandlerWithHistoryAccessAndObservability(
			*frontend,
			historyStore,
			accessValidator,
			observabilityQuery,
			*miniDeployURL,
			*miniBaseURL,
		)
	}

	servers := []struct {
		name   string
		server *http.Server
	}{
		{
			name: "private",
			server: &http.Server{
				Addr:    *listen,
				Handler: privateHandler,
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

	if observabilityDone != nil {
		<-observabilityDone
	}
	if recoveryDiscordDone != nil {
		<-recoveryDiscordDone
	}
	if observabilityStore != nil {
		if err := observabilityStore.Close(); err != nil {
			log.Printf("close historical observability: %v", err)
		}
	}

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
