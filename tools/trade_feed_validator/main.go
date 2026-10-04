package main

import (
	"fmt"
	"os"
	"trade_feed_validator"
)

func main() {
	filePath := "sample_feed.csv"
	if len(os.Args) > 1 {
		filePath = os.Args[1]
	}

	file, err := os.Open(filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening file %s: %v\n", filePath, err)
		os.Exit(1)
	}
	defer file.Close()

	result, err := trade_feed_validator.ValidateFeed(file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Validation error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("=== Validation Results for %s ===\n", filePath)
	fmt.Printf("Accepted Valid Events: %d\n", len(result.ValidEvents))
	for _, e := range result.ValidEvents {
		fmt.Printf("  [VALID] EventID: %s, TxHash: %s, Amount: %s\n", e.EventID, e.TxHash, e.Amount)
	}

	fmt.Printf("\nRejected DLQ Events: %d\n", len(result.DLQEvents))
	for _, d := range result.DLQEvents {
		fmt.Printf("  [DLQ] Reason: %s | EventID: %s\n", d.Reason, d.Event.EventID)
	}
}
