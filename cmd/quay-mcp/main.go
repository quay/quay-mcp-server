package main

import (
	"flag"
	"log"
	"os"

	"github.com/quay/quay-mcp-server/internal/server"
)

func main() {
	url := flag.String("url", "", "Quay registry URL (required)")
	token := flag.String("token", "", "OAuth token (or set QUAY_OAUTH_TOKEN)")
	flag.Parse()

	if *url == "" {
		log.Fatal("-url is required")
	}

	oauthToken := *token
	if oauthToken == "" {
		oauthToken = os.Getenv("QUAY_OAUTH_TOKEN")
	}

	s := server.NewQuayMCPServer(*url, oauthToken)
	if err := s.Start(); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
