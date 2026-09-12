// Package main provides the binary entrypoint for the server application.
package main

import (
	"log"

	"AshitomW/mini-lambda/internal/app"
)

func main() {
	if err := app.Run(); err != nil {
		log.Fatalf("Server terminated with error: %v", err)
	}
}
