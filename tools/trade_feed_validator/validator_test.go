package main

import (
	"os"
	"strings"
	"testing"
)

func TestValidateFeed_MalformedCSVAndDLQ(t *testing.T) {
	csvData := `event_id,tx_hash,block_time,wallet,side,amount,ingested_at
evt_001,0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef,10:00:00,0xWalletA,BUY,1.5,10:00:05
evt_002,"unclosed_quote_row,0xabcdef,10:01:00,0xWalletB,SELL,10.0,10:01:02
`

	result, err := ValidateFeed(strings.NewReader(csvData))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.ValidEvents) != 1 {
		t.Errorf("expected 1 valid event, got %d", len(result.ValidEvents))
	}

	if len(result.DLQEvents) != 1 {
		t.Fatalf("expected 1 DLQ event, got %d", len(result.DLQEvents))
	}

	if result.DLQEvents[0].RawInput == "" {
		t.Errorf("expected RawInput to be preserved for malformed CSV parse error")
	}
}

func TestValidateFeed_SampleFeedFile(t *testing.T) {
	file, err := os.Open("sample_feed.csv")
	if err != nil {
		t.Skip("sample_feed.csv not found, skipping file test")
		return
	}
	defer file.Close()

	result, err := ValidateFeed(file)
	if err != nil {
		t.Fatalf("failed to validate sample_feed.csv: %v", err)
	}

	if len(result.ValidEvents) == 0 && len(result.DLQEvents) == 0 {
		t.Errorf("expected non-empty validation result for sample_feed.csv")
	}
}
