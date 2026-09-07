package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"

	"github.com/conradevans/ReactorLab/internal/api"
)

func main() {
	listen := flag.String("listen", "127.0.0.1:9200", "HTTP listen address")
	flag.Parse()

	host, _, err := net.SplitHostPort(*listen)
	if err != nil {
		log.Fatal(err)
	}

	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		log.Fatal("ReactorLab private listener must use a loopback address")
	}

	server := &http.Server{
		Addr:    *listen,
		Handler: api.NewHandler(),
	}

	fmt.Printf("ReactorLab listening on %s\n", *listen)
	log.Fatal(server.ListenAndServe())
}
