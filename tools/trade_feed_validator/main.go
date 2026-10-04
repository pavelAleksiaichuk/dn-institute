package main

import (
	"encoding/json"
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run . sample_feed.csv")
		os.Exit(1)
	}

	file, err := os.Open(os.Args[1])
	if err != nil {
		fmt.Printf("Failed to open file: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()

	result, err := ValidateFeed(file)
	if err != nil {
		fmt.Printf("Validation error: %v\n", err)
		os.Exit(1)
	}

	dlqData, err := json.MarshalIndent(result.DLQEvents, "", "  ")
	if err == nil {
		_ = os.WriteFile("dlq_report.json", dlqData, 0644)
	}

	validData, err := json.MarshalIndent(result.ValidEvents, "", "  ")
	if err == nil {
		_ = os.WriteFile("valid_report.json", validData, 0644)
	}

	fmt.Printf("Successfully validated events: %d (saved to valid_report.json)\n", len(result.ValidEvents))
	fmt.Printf("Sent to DLQ: %d (persisted to dlq_report.json)\n", len(result.DLQEvents))
}
