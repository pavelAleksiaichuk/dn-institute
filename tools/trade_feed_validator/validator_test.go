package main

import (
	"strings"
	"testing"
)

func TestValidateFeed_ValidScenario(t *testing.T) {
	csvData := `event_id,tx_hash,block_time,wallet,side,amount,ingested_at
evt_001,0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef,10:00:00,0xWalletA,BUY,1.5,10:00:05
evt_002,0xabcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890ab,10:01:00,0xWalletB,SELL,10.0,10:01:02
evt_003,0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef,10:02:00,0xWalletC,BUY,0.5,10:02:01
evt_004,0x1111111111111111111111111111111111111111111111111111111111111111,10:03:00,0xWalletD,BUY,-100.0,10:03:05
evt_005,0x2222222222222222222222222222222222222222222222222222222222222222,,0xWalletE,BUY,1.0,10:04:05
evt_006,0x3333333333333333333333333333333333333333333333333333333333333333,10:05:00,0xWalletF,BUY,1.0,10:05:05
evt_007,0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef,10:06:00,0xWalletG,BUY,2.0,10:06:01
evt_008,0x4444444444444444444444444444444444444444444444444444444444444444,10:07:00,0xWalletH,BUY,1.0,10:07:05
`

	result, err := ValidateFeed(strings.NewReader(csvData))
	if err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}

	if len(result.ValidEvents) != 4 {
		t.Errorf("expected 4 valid events, got %d", len(result.ValidEvents))
	}

	if len(result.DLQEvents) != 4 {
		t.Errorf("expected 4 DLQ events, got %d", len(result.DLQEvents))
	}

	expectedDLQReasons := map[string]string{
		"evt_003": "duplicate transaction hash",
		"evt_004": "invalid amount format, non-positive value, or non-finite number",
		"evt_005": "missing mandatory field",
		"evt_007": "duplicate transaction hash",
	}

	for _, item := range result.DLQEvents {
		if item.Event == nil {
			continue
		}
		eventID := item.Event.EventID
		if prefix, exists := expectedDLQReasons[eventID]; exists {
			if !strings.Contains(item.Reason, prefix) {
				t.Errorf("for event %s expected reason containing '%s', got '%s'", eventID, prefix, item.Reason)
			}
		}
	}
}

func TestValidateFeed_ClockSkew(t *testing.T) {
	csvData := `event_id,tx_hash,block_time,wallet,side,amount,ingested_at
evt_008,0x4444444444444444444444444444444444444444444444444444444444444444,10:07:05,0xWalletH,BUY,1.0,10:07:00
`

	result, err := ValidateFeed(strings.NewReader(csvData))
	if err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}

	if len(result.DLQEvents) != 1 {
		t.Fatalf("expected 1 DLQ event, got %d", len(result.DLQEvents))
	}

	if !strings.Contains(result.DLQEvents[0].Reason, "clock skew detected") {
		t.Errorf("expected clock skew reason, got %s", result.DLQEvents[0].Reason)
	}
}
