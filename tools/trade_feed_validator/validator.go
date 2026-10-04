package main

import (
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

const TimeFormat = "15:04:05"

type TradeEvent struct {
	EventID          string    `json:"event_id"`
	Symbol           string    `json:"symbol"`
	Price            float64   `json:"price"`
	Quantity         float64   `json:"quantity"`
	Side             string    `json:"side"`
	BlockTime        string    `json:"block_time"`
	IngestTime       string    `json:"ingest_time"`
	TxHash           string    `json:"tx_hash"`
	ParsedBlockTime  time.Time `json:"-"`
	ParsedIngestTime time.Time `json:"-"`
}

type DLQItem struct {
	Event  TradeEvent `json:"event"`
	Reason string     `json:"reason"`
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

	expectedHeader := []string{"event_id", "symbol", "price", "quantity", "side", "block_time", "ingest_time", "tx_hash"}
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
					Event:  TradeEvent{},
					Reason: fmt.Sprintf("malformed csv row (line %d): %v", lineNum, parseErr),
				})
				lineNum++
				continue
			}
			return nil, err
		}
		lineNum++

		if len(record) < 8 {
			result.DLQEvents = append(result.DLQEvents, DLQItem{
				Event:  TradeEvent{},
				Reason: "malformed row: insufficient columns",
			})
			continue
		}

		eventID := strings.TrimSpace(record[0])
		symbol := strings.TrimSpace(record[1])
		priceStr := strings.TrimSpace(record[2])
		qtyStr := strings.TrimSpace(record[3])
		side := strings.TrimSpace(record[4])
		blockTimeStr := strings.TrimSpace(record[5])
		ingestTimeStr := strings.TrimSpace(record[6])
		txHash := strings.TrimSpace(record[7])

		if eventID == "" || symbol == "" || side == "" || blockTimeStr == "" || ingestTimeStr == "" || txHash == "" {
			result.DLQEvents = append(result.DLQEvents, DLQItem{
				Event:  TradeEvent{EventID: eventID, Symbol: symbol, Side: side, TxHash: txHash},
				Reason: "missing mandatory field",
			})
			continue
		}

		price, err := strconv.ParseFloat(priceStr, 64)
		if err != nil || price <= 0 {
			result.DLQEvents = append(result.DLQEvents, DLQItem{
				Event:  TradeEvent{EventID: eventID, Symbol: symbol, Side: side, TxHash: txHash},
				Reason: "invalid price format or non-positive value",
			})
			continue
		}

		quantity, err := strconv.ParseFloat(qtyStr, 64)
		if err != nil || quantity <= 0 {
			result.DLQEvents = append(result.DLQEvents, DLQItem{
				Event:  TradeEvent{EventID: eventID, Symbol: symbol, Side: side, TxHash: txHash},
				Reason: "invalid quantity format or non-positive value",
			})
			continue
		}

		upperSide := strings.ToUpper(side)
		if upperSide != "BUY" && upperSide != "SELL" {
			result.DLQEvents = append(result.DLQEvents, DLQItem{
				Event:  TradeEvent{EventID: eventID, Symbol: symbol, Side: side, TxHash: txHash},
				Reason: "invalid side value",
			})
			continue
		}

		parsedBlockTime, err := time.Parse(TimeFormat, blockTimeStr)
		if err != nil {
			result.DLQEvents = append(result.DLQEvents, DLQItem{
				Event:  TradeEvent{EventID: eventID, Symbol: symbol, Side: side, TxHash: txHash},
				Reason: "invalid time format (expected HH:MM:SS)",
			})
			continue
		}

		parsedIngestTime, err := time.Parse(TimeFormat, ingestTimeStr)
		if err != nil {
			result.DLQEvents = append(result.DLQEvents, DLQItem{
				Event:  TradeEvent{EventID: eventID, Symbol: symbol, Side: side, TxHash: txHash},
				Reason: "invalid time format (expected HH:MM:SS)",
			})
			continue
		}

		event := TradeEvent{
			EventID:          eventID,
			Symbol:           symbol,
			Price:            price,
			Quantity:         quantity,
			Side:             upperSide,
			BlockTime:        blockTimeStr,
			IngestTime:       ingestTimeStr,
			TxHash:           txHash,
			ParsedBlockTime:  parsedBlockTime,
			ParsedIngestTime: parsedIngestTime,
		}

		if parsedIngestTime.Before(parsedBlockTime) {
			result.DLQEvents = append(result.DLQEvents, DLQItem{
				Event:  event,
				Reason: "clock skew detected: ingested_at is before block_time",
			})
			continue
		}

		canonicalHash := event.TxHash
		if decoded, err := hex.DecodeString(event.TxHash); err == nil {
			canonicalHash = hex.EncodeToString(decoded)
		} else {
			canonicalHash = strings.ToLower(event.TxHash)
		}

		if prevEventID, exists := seenTxHash[canonicalHash]; exists {
			result.DLQEvents = append(result.DLQEvents, DLQItem{
				Event:  event,
				Reason: fmt.Sprintf("duplicate transaction hash (already seen in event %s)", prevEventID),
			})
			continue
		}

		seenTxHash[canonicalHash] = event.EventID
		result.ValidEvents = append(result.ValidEvents, event)
	}

	return result, nil
}
