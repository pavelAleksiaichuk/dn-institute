package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run main.go <path_to_csv>")
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

	fmt.Printf("Successfully validated events: %d\n", len(result.ValidEvents))
	fmt.Printf("Sent to DLQ: %d\n", len(result.DLQEvents))
}
