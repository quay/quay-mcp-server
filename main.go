package main

import (
	"flag"
	"log"
	"os"
)

func main() {
	var registryURL string
	var oauthToken string

	// Define command-line flags
	flag.StringVar(&registryURL, "url", "", "Quay registry URL (required)")
	flag.StringVar(&oauthToken, "token", "", "OAuth token for authentication (optional)")

	// Custom usage message
	flag.Usage = func() {
		log.Printf("Usage: %s -url <quay-registry-url> [-token <oauth-token>]\n", os.Args[0])
		log.Println("\nFlags:")
		flag.PrintDefaults()
		log.Println("\nExamples:")
		log.Printf("  %s -url https://quay.io\n", os.Args[0])
		log.Printf("  %s -url https://quay.io -token your-oauth-token\n", os.Args[0])
	}

	// Parse command-line flags
	flag.Parse()

	// Validate required arguments
	if registryURL == "" {
		log.Println("Error: registry URL is required")
		flag.Usage()
		os.Exit(1)
	}

	server := NewQuayMCPServer(registryURL, oauthToken)

	if err := server.Start(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
