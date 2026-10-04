package main

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

type TradeEvent struct {
	EventID    string
	TxHash     string
	BlockTime  string
	Wallet     string
	Side       string
	Amount     string
	IngestedAt string
	ParsedBlockTime time.Time
	ParsedIngestTime time.Time
}

type DLQItem struct {
	Event  TradeEvent
	Reason string
}

type ValidationResult struct {
	ValidEvents []TradeEvent
	DLQEvents   []DLQItem
}

const TimeFormat = "15:04:05"

func ValidateFeed(r io.Reader) (*ValidationResult, error) {
	reader := csv.NewReader(r)
	
	header, err := reader.Read()
	if err != nil {
		if err == io.EOF {
			return &ValidationResult{}, nil
		}
		return nil, fmt.Errorf("failed to read header: %w", err)
	}
	_ = header

	seenTxHash := make(map[string]string)
	result := &ValidationResult{}

	lineNum := 1
	for {
		lineNum++
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("csv read error at line %d: %w", lineNum, err)
		}

		if len(record) < 7 {
			result.DLQEvents = append(result.DLQEvents, DLQItem{
				Event:  TradeEvent{EventID: fmt.Sprintf("line_%d", lineNum)},
				Reason: fmt.Sprintf("malformed row: expected 7 columns, got %d", len(record)),
			})
			continue
		}

		event := TradeEvent{
			EventID:    strings.TrimSpace(record[0]),
			TxHash:     strings.TrimSpace(record[1]),
			BlockTime:  strings.TrimSpace(record[2]),
			Wallet:     strings.TrimSpace(record[3]),
			Side:       strings.TrimSpace(record[4]),
			Amount:     strings.TrimSpace(record[5]),
			IngestedAt: strings.TrimSpace(record[6]),
		}

		if event.BlockTime == "" || strings.EqualFold(event.BlockTime, "null") {
			result.DLQEvents = append(result.DLQEvents, DLQItem{
				Event:  event,
				Reason: "missing mandatory block_time",
			})
			continue
		}

		parsedBlockTime, err1 := time.Parse(TimeFormat, event.BlockTime)
		parsedIngestTime, err2 := time.Parse(TimeFormat, event.IngestedAt)
		if err1 != nil || err2 != nil {
			result.DLQEvents = append(result.DLQEvents, DLQItem{
				Event:  event,
				Reason: "invalid time format (expected HH:MM:SS)",
			})
			continue
		}
		event.ParsedBlockTime = parsedBlockTime
		event.ParsedIngestTime = parsedIngestTime

		if prevEventID, exists := seenTxHash[event.TxHash]; exists {
			result.DLQEvents = append(result.DLQEvents, DLQItem{
				Event:  event,
				Reason: fmt.Sprintf("duplicate tx_hash of %s", prevEventID),
			})
			continue
		}
		seenTxHash[event.TxHash] = event.EventID

		if event.ParsedIngestTime.Before(event.ParsedBlockTime) {
			result.DLQEvents = append(result.DLQEvents, DLQItem{
				Event:  event,
				Reason: "ingested_at is earlier than block_time (clock skew)",
			})
			continue
		}

		result.ValidEvents = append(result.ValidEvents, event)
	}

	return result, nil
}

func main() {
	file, err := os.Open("sample_feed.csv")
	if err != nil {
		fmt.Printf("Error opening file: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()

	res, err := ValidateFeed(file)
	if err != nil {
		fmt.Printf("Error processing feed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Pipeline executed successfully.\n")
	fmt.Printf("Valid events: %d\n", len(res.ValidEvents))
	fmt.Printf("DLQ rejected events: %d\n", len(res.DLQEvents))
	for _, item := range res.DLQEvents {
		fmt.Printf(" - [%s] Rejected: %s\n", item.Event.EventID, item.Reason)
	}
}
