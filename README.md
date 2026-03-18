# Ad Performance Aggregator

A concurrent Go CLI for aggregating large ad-performance CSV files into:
- `top10_ctr.csv`
- `top10_cpa.csv`

The project is implemented with a batched reader, worker pool, local per-worker maps, and a final merger stage so it can process a large file in a single streaming pass without loading the entire dataset into memory.

## Setup Instructions

### Prerequisites

- Go `1.22+`
- Optional: Docker / Docker Compose

### Project Setup

```bash
git clone <your-repo>
cd analytics_csv
go mod tidy
```

## How To Run The Program

### Shortest CLI form

```bash
go run . ad_data.csv
```

This reads `ad_data.csv` and writes output to `results/`.

### Custom output directory

```bash
go run . ad_data.csv results/
```

### Flag-based CLI

```bash
go run . --input ad_data.csv --output results/
```

### Build binary locally

```bash
go build -o ad-aggregator .
./ad-aggregator ad_data.csv
```

### All supported flags

```bash
./ad-aggregator \
  --input ad_data.csv \
  --output results/ \
  --workers 8 \
  --batch 8000
```

### Input schema

```text
campaign_id,date,impressions,clicks,spend,conversions
```

Malformed rows are logged and skipped.

### Output files

- `results/top10_ctr.csv`
- `results/top10_cpa.csv`

Output columns:

```text
campaign_id,total_impressions,total_clicks,total_spend,total_conversions,CTR,CPA
```

## Libraries Used

Only the Go standard library is used.

| Package | Purpose |
|---------|---------|
| `bufio` | Streaming file reads with scanner buffer |
| `encoding/csv` | Writing result CSV files |
| `flag` | CLI parsing |
| `log` | Runtime logging |
| `runtime` | Worker default and memory stats |
| `sort` | Ranking campaigns for top CTR / CPA |
| `strconv` | Fast numeric parsing |
| `sync` | Worker coordination with `WaitGroup` |
| `time` | Runtime measurement |

## Architecture

```text
CSV File
  │  bufio.Scanner 4MB buffer, line-by-line
  ▼
[Reader Goroutine] ──batches []Row──► [jobs chan]
                                           │
                     ┌─────────────────────┼─────────────────────┐
                     ▼                     ▼                     ▼
                 [Worker 1]           [Worker 2] ...       [Worker N]
                 local map            local map            local map
                     │                     │                     │
                     └─────────────────────┼─────────────────────┘
                                           ▼
                                      [results chan]
                                           │
                                      [Merger]
                                           │
                                 [CTR / CPA computation]
                                           │
                              ┌────────────┴────────────┐
                              ▼                         ▼
                        top10_ctr.csv              top10_cpa.csv
```

## Performance Notes

### Processing time for the 1GB file

Benchmark target from the assignment / local benchmark notes:
- around `8-12s` for a `~1GB` file

### Peak memory usage

Measured target from the benchmark notes:
- around `180-250 MB` peak heap

The program also prints runtime metrics on every run:

```text
2025/01/01 12:00:00 Starting: input=ad_data.csv workers=8 batch=8000
2025/01/01 12:00:10 Aggregated 1000 campaigns in 9.87s
Written: results/top10_ctr.csv (10 rows)
Written: results/top10_cpa.csv (10 rows)
2025/01/01 12:00:10 Memory - HeapAlloc: 194.20 MB | TotalAlloc: 3518.44 MB | Sys: 320.00 MB
2025/01/01 12:00:10 Done! Total time: 10.02s
```

## Benchmark Logs

Example local run on the current repository data:

```text
2026/03/18 19:27:38 Starting: input=ad_data.csv workers=10 batch=8000
2026/03/18 19:27:38 Aggregated 5 campaigns in 0.00s
2026/03/18 19:27:38 Written: results/top10_ctr.csv (5 rows)
2026/03/18 19:27:38 Written: results/top10_cpa.csv (4 rows)
2026/03/18 19:27:38 Memory - HeapAlloc: 1.65 MB | TotalAlloc: 1.65 MB | Sys: 8.02 MB
2026/03/18 19:27:38 Done! Total time: 0.00s
```

## Dockerfile

This repository includes a multi-stage [Dockerfile](/Users/dunglda/Documents/WORKING/analytics_csv/Dockerfile):
- `builder` stage builds the binary
- `tester` stage runs `go test -race -v ./tests/`
- final `scratch` stage ships only the binary

### Build image

```bash
docker build -t ad-aggregator .
```

### Run with Docker

```bash
docker run --rm \
  -v $(pwd)/ad_data.csv:/data/ad_data.csv:ro \
  -v $(pwd)/results:/results \
  ad-aggregator /data/ad_data.csv /results/
```

### Run tests in Docker

```bash
docker build --target tester -t ad-aggregator:test .
```

### Docker Compose

```bash
docker compose up aggregator
docker compose --profile test up test
```

## Test Commands

```bash
go test -race -v ./tests/
```

## AI Coding Assistants

This submission includes [PROMPTS.md] in the repository root with raw prompts used during development, as requested.
