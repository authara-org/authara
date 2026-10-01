package main

import (
	"context"
	"log"
	"os"

	"github.com/authara-org/authara/internal/lifecycle"
	"github.com/authara-org/authara/internal/operations"
)

var Version = "dev"

func main() {
	// Local readiness probe for Docker HEALTHCHECK.
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		if err := lifecycle.RunHealthcheck(context.Background()); err != nil {
			os.Exit(1)
		}
		return
	}
	if len(os.Args) > 1 && operations.IsCommand(os.Args[1]) {
		if err := operations.Run(context.Background(), os.Args[1:], os.Stdout); err != nil {
			log.Fatalf("operational command failed: %v", err)
		}
		return
	}

	if err := lifecycle.Run(Version); err != nil {
		log.Printf("authara failed: %v", err)
		os.Exit(1)
	}
}
