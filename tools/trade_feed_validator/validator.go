package main

import (
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"time"
)

const TimeFormat = "15:04:05"

type TradeEvent struct {
	EventID          string    `json:"event_id"`
	TxHash           string    `json:"tx_hash"`
	BlockTime        string    `json:"block_time"`
	Wallet           string    `json:"wallet"`
	Side             string    `json:"side"`
	Amount           float64   `json:"amount"`
	IngestedAt       string    `json:"ingested_at"`
	ParsedBlockTime  time.Time `json:"-"`
	ParsedIngestTime time.Time `json:"-"`
}

type DLQItem struct {
	RawRecord []string    `json:"raw_record"`
	Event     *TradeEvent `json:"event,omitempty"`
	Reason    string      `json:"reason"`
}

type ValidationResult struct {
	ValidEvents []TradeEvent `json:"valid_events"`
	DLQEvents   []DLQItem    `json:"dlq_events"`
}

func ValidateFeed(r io.Reader) (*ValidationResult, error) {
	reader := csv.NewReader(r)

	header, err := reader.Read()
	if err != nil {
		return nil, err
	}

	expectedHeader := []string{"event_id", "tx_hash", "block_time", "wallet", "side", "amount", "ingested_at"}
	if len(header) != len(expectedHeader) {
		return nil, fmt.Errorf("invalid header length: expected %d, got %d", len(expectedHeader), len(header))
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
			if parseErr, ok := err.(*csv.ParseError); ok {
				result.DLQEvents = append(result.DLQEvents, DLQItem{
					RawRecord: nil,
					Reason:    fmt.Sprintf("malformed csv row (line %d): %v", lineNum, parseErr),
				})
				lineNum++
				continue
			}
			return nil, err
		}
		lineNum++

		if len(record) < 7 {
			result.DLQEvents = append(result.DLQEvents, DLQItem{
				RawRecord: record,
				Reason:    "malformed row: insufficient columns",
			})
			continue
		}

		eventID := strings.TrimSpace(record[0])
		txHash := strings.TrimSpace(record[1])
		blockTimeStr := strings.TrimSpace(record[2])
		wallet := strings.TrimSpace(record[3])
		side := strings.TrimSpace(record[4])
		amountStr := strings.TrimSpace(record[5])
		ingestedAtStr := strings.TrimSpace(record[6])

		tempEvent := &TradeEvent{
			EventID:    eventID,
			TxHash:     txHash,
			BlockTime:  blockTimeStr,
			Wallet:     wallet,
			Side:       side,
			IngestedAt: ingestedAtStr,
		}

		if eventID == "" || txHash == "" || blockTimeStr == "" || wallet == "" || side == "" || ingestedAtStr == "" {
			result.DLQEvents = append(result.DLQEvents, DLQItem{
				RawRecord: record,
				Event:     tempEvent,
				Reason:    "missing mandatory field",
			})
			continue
		}

		amount, err := strconv.ParseFloat(amountStr, 64)
		if err != nil || amount <= 0 || math.IsNaN(amount) || math.IsInf(amount, 0) {
			result.DLQEvents = append(result.DLQEvents, DLQItem{
				RawRecord: record,
				Event:     tempEvent,
				Reason:    "invalid amount format, non-positive value, or non-finite number",
			})
			continue
		}
		tempEvent.Amount = amount

		upperSide := strings.ToUpper(side)
		if upperSide != "BUY" && upperSide != "SELL" {
			result.DLQEvents = append(result.DLQEvents, DLQItem{
				RawRecord: record,
				Event:     tempEvent,
				Reason:    "invalid side value",
			})
			continue
		}
		tempEvent.Side = upperSide

		parsedBlockTime, err := time.Parse(TimeFormat, blockTimeStr)
		if err != nil {
			result.DLQEvents = append(result.DLQEvents, DLQItem{
				RawRecord: record,
				Event:     tempEvent,
				Reason:    "invalid time format (expected HH:MM:SS)",
			})
			continue
		}

		parsedIngestTime, err := time.Parse(TimeFormat, ingestedAtStr)
		if err != nil {
			result.DLQEvents = append(result.DLQEvents, DLQItem{
				RawRecord: record,
				Event:     tempEvent,
				Reason:    "invalid time format (expected HH:MM:SS)",
			})
			continue
		}

		tempEvent.ParsedBlockTime = parsedBlockTime
		tempEvent.ParsedIngestTime = parsedIngestTime

		if parsedIngestTime.Before(parsedBlockTime) {
			result.DLQEvents = append(result.DLQEvents, DLQItem{
				RawRecord: record,
				Event:     tempEvent,
				Reason:    "clock skew detected: ingested_at is before block_time",
			})
			continue
		}

		canonicalHash := txHash
		if decoded, err := hex.DecodeString(txHash); err == nil {
			canonicalHash = hex.EncodeToString(decoded)
		} else {
			canonicalHash = strings.ToLower(txHash)
		}

		if prevEventID, exists := seenTxHash[canonicalHash]; exists {
			result.DLQEvents = append(result.DLQEvents, DLQItem{
				RawRecord: record,
				Event:     tempEvent,
				Reason:    fmt.Sprintf("duplicate transaction hash (already seen in event %s)", prevEventID),
			})
			continue
		}

		seenTxHash[canonicalHash] = eventID
		result.ValidEvents = append(result.ValidEvents, *tempEvent)
	}

	return result, nil
}
