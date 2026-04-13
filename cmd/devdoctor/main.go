package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"idea/internal/config"
	"idea/internal/devdoctor"
)

func main() {
	jsonOutput := flag.Bool("json", false, "print doctor report as json")
	flag.Parse()

	cfg := config.Load()
	service := devdoctor.Service{
		Config:         cfg,
		WorkerProvider: getenv("IDEA_WORKER_PROVIDER", "placeholder"),
	}
	report := service.Run(context.Background())

	if *jsonOutput {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		_ = encoder.Encode(report)
	} else {
		fmt.Printf("status: %s\n", report.Status)
		for _, item := range report.Checks {
			fmt.Printf("- %-16s %-10s %s\n", item.Name, item.Status, item.Message)
		}
	}

	if report.Status == "not_ready" {
		os.Exit(1)
	}
}

func getenv(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
