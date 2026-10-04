# Trade Feed Validator

## Overview
A lightweight, robust Go-based validator designed to inspect trade event feeds from blockchain indexers and prevent corrupted data from poisoning downstream analytics (Volume, VWAP, wallet clustering).

## Data-Quality Issues Found in `sample_feed.csv`

1. **Duplicate Transactions (`evt_003`, `evt_007`)**
   - **Affected Rows:** `evt_003` (duplicate of `evt_002` with hash `0xaa2`), `evt_007` (duplicate of `evt_006` with hash `0xaa5`).
   - **Downstream Impact:** Double-counts trade volume and inflates active wallet metrics.

2. **Missing Critical Field (`evt_005`)**
   - **Affected Rows:** `evt_005` (`block_time` is `null`).
   - **Downstream Impact:** Breaks chronological sorting, window functions, and distorts VWAP (Volume Weighted Average Price) calculations because price/time alignment is lost.
   - **Handling Strategy for `evt_005`:** Sent directly to a **Dead-Letter Queue (DLQ)** with a `missing mandatory block_time` reason tag. We drop it from the primary analytics ingestion table to protect data integrity. *What would change this?* If the source indexer supported historical state recovery or event replays via RPC, we would trigger an asynchronous backfill query instead of permanent dropping.

3. **Time Travel / Clock Skew Anomaly (`evt_008`)**
   - **Affected Rows:** `evt_008` (`block_time` = `10:10:00`, but `ingested_at` = `09:59:50`).
   - **Downstream Impact:** Violates causal integrity. Ingested time cannot precede block time; this breaks time-series sliding windows and real-time streaming aggregations.

## Recommended Production Safeguards
To scale these checks and prevent data quality issues globally across the pipeline in a production environment, future iterations could integrate:
1. **Schema Validation Gateways:** Enforce strict contract definitions (Protobuf/JSON Schema) at the ingestion API boundary.
2. **Distributed Deduplication:** Scale beyond in-memory state by utilizing Redis-backed sliding windows or database unique index constraints on `tx_hash` for cross-instance deduplication.
3. **Observability & Alerting:** Hook the DLQ outputs into monitoring systems (e.g., Prometheus/Grafana) to trigger automated alerts on high anomaly spikes or clock-skew rates.

## How to Run
1. Navigate to the tool directory:
   ```bash
   cd tools/trade_feed_validator
   ```
2. Run tests:
   ```bash
   go test -v .
   ```
3. Run the validator:
   ```bash
   go run . sample_feed.csv
   ```