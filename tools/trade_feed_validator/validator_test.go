package main

import (
	"strings"
	"testing"
)

func TestValidateFeed(t *testing.T) {
	csvData := `event_id,tx_hash,block_time,wallet,side,amount,ingested_at
evt_001,0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef,10:00:00,0xWalletA,BUY,1.5,10:00:05
evt_002,0xabcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890ab,10:01:00,0xWalletB,SELL,10.0,10:01:02
evt_003,0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef,10:02:00,0xWalletC,BUY,0.5,10:02:01
evt_005,,10:03:00,0xWalletE,BUY,1.0,10:03:05
evt_007,0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef,10:04:00,0xWalletG,BUY,2.0,10:04:01
evt_008,0x3333333333333333333333333333333333333333333333333333333333333333,10:05:05,0xWalletH,BUY,1.0,10:05:00
`

	result, err := ValidateFeed(strings.NewReader(csvData))
	if err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}

	if len(result.ValidEvents) != 2 {
		t.Errorf("expected 2 valid events, got %d", len(result.ValidEvents))
	}

	expectedValidIDs := map[string]bool{"evt_001": true, "evt_002": true}
	for _, ev := range result.ValidEvents {
		if !expectedValidIDs[ev.EventID] {
			t.Errorf("unexpected valid event ID: %s", ev.EventID)
		}
	}

	expectedDLQReasons := map[string]string{
		"evt_003": "duplicate transaction hash",
		"evt_005": "missing mandatory field",
		"evt_007": "duplicate transaction hash",
		"evt_008": "clock skew detected",
	}

	if len(result.DLQEvents) != len(expectedDLQReasons) {
		t.Errorf("expected %d DLQ events, got %d", len(expectedDLQReasons), len(result.DLQEvents))
	}

	for _, item := range result.DLQEvents {
		if item.Event == nil {
			t.Errorf("DLQ item missing event context")
			continue
		}
		eventID := item.Event.EventID
		prefix, exists := expectedDLQReasons[eventID]
		if !exists {
			t.Errorf("unexpected DLQ event ID: %s", eventID)
			continue
		}
		if !strings.Contains(item.Reason, prefix) {
			t.Errorf("for event %s expected reason containing '%s', got '%s'", eventID, prefix, item.Reason)
		}
	}
}
