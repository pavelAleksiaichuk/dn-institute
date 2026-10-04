package main

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

type TradeEvent struct {
	EventID          string
	TxHash           string
	BlockTime        string
	Wallet           string
	Side             string
	Amount           string
	IngestedAt       string
	ParsedBlockTime  time.Time
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

var expectedHeader = []string{"event_id", "tx_hash", "block_time", "wallet", "side", "amount", "ingested_at"}

func ValidateFeed(r io.Reader) (*ValidationResult, error) {
	reader := csv.NewReader(r)

	header, err := reader.Read()
	if err != nil {
		if err == io.EOF {
			return &ValidationResult{}, nil
		}
		return nil, fmt.Errorf("failed to read header: %w", err)
	}

	if len(header) != len(expectedHeader) {
		return nil, fmt.Errorf("invalid header column count: expected %d, got %d", len(expectedHeader), len(header))
	}
	for i, h := range header {
		cleanHeader := strings.TrimSpace(strings.ToLower(h))
		if cleanHeader != expectedHeader[i] {
			return nil, fmt.Errorf("invalid header at index %d: expected %s, got %s", i, expectedHeader[i], cleanHeader)
		}
	}

	result := &ValidationResult{}
	seenTxHash := make(map[string]string)
	lineNum := 1

	for {
		record, err := reader.Read()
		if err != nil {
			if err == io.EOF {
				break
			}
			lineNum++
			if parseErr, ok := err.(*csv.ParseError); ok && parseErr.Err == csv.ErrFieldCount {
				result.DLQEvents = append(result.DLQEvents, DLQItem{
					Event:  TradeEvent{},
					Reason: fmt.Sprintf("malformed row: field count mismatch at line %d", lineNum),
				})
				continue
			}
			return nil, fmt.Errorf("csv read error at line %d: %w", lineNum, err)
		}
		lineNum++

		if len(record) != 7 {
			result.DLQEvents = append(result.DLQEvents, DLQItem{
				Event: TradeEvent{
					EventID:    safeGet(record, 0),
					TxHash:     safeGet(record, 1),
					BlockTime:  safeGet(record, 2),
					Wallet:     safeGet(record, 3),
					Side:       safeGet(record, 4),
					Amount:     safeGet(record, 5),
					IngestedAt: safeGet(record, 6),
				},
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

		if event.EventID == "" || event.TxHash == "" || event.Wallet == "" || event.Side == "" || event.Amount == "" {
			result.DLQEvents = append(result.DLQEvents, DLQItem{
				Event:  event,
				Reason: "missing required fields",
			})
			continue
		}

		upperSide := strings.ToUpper(event.Side)
		if upperSide != "BUY" && upperSide != "SELL" {
			result.DLQEvents = append(result.DLQEvents, DLQItem{
				Event:  event,
				Reason: fmt.Sprintf("unsupported side value: %s", event.Side),
			})
			continue
		}

		amountVal, err := strconv.ParseFloat(event.Amount, 64)
		if err != nil || amountVal <= 0 {
			result.DLQEvents = append(result.DLQEvents, DLQItem{
				Event:  event,
				Reason: fmt.Sprintf("invalid amount: %s", event.Amount),
			})
			continue
		}

		parsedBlockTime, err := time.Parse(TimeFormat, event.BlockTime)
		if err != nil {
			result.DLQEvents = append(result.DLQEvents, DLQItem{
				Event:  event,
				Reason: "invalid time format (expected HH:MM:SS)",
			})
			continue
		}

		parsedIngestTime, err := time.Parse(TimeFormat, event.IngestedAt)
		if err != nil {
			result.DLQEvents = append(result.DLQEvents, DLQItem{
				Event:  event,
				Reason: "invalid time format (expected HH:MM:SS)",
			})
			continue
		}

		event.ParsedBlockTime = parsedBlockTime
		event.ParsedIngestTime = parsedIngestTime

		if parsedIngestTime.Before(parsedBlockTime) {
			result.DLQEvents = append(result.DLQEvents, DLQItem{
				Event:  event,
				Reason: "clock skew detected: ingested_at is before block_time",
			})
			continue
		}

		if prevEventID, exists := seenTxHash[event.TxHash]; exists {
			result.DLQEvents = append(result.DLQEvents, DLQItem{
				Event:  event,
				Reason: fmt.Sprintf("duplicate tx_hash of %s", prevEventID),
			})
			continue
		}

		seenTxHash[event.TxHash] = event.EventID
		result.ValidEvents = append(result.ValidEvents, event)
	}

	return result, nil
}

func safeGet(record []string, idx int) string {
	if idx >= 0 && idx < len(record) {
		return strings.TrimSpace(record[idx])
	}
	return ""
}
