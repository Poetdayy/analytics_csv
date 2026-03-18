# Ad Performance Aggregator

A concurrent Go CLI for aggregating large ad-performance CSV files into:
- `top10_ctr.csv`
- `top10_cpa.csv`

The implementation uses a streaming reader, batched worker pool, local per-worker maps, and a final merger so it can process a large CSV in a single pass without loading the full file into memory.

## Quick Start With Docker

This repository is set up so another machine can run it with Docker only.

### 1. Build the image

```bash
docker build -t ad-aggregator .
```

### 2. Run with the sample CSV included in this repo

```bash
mkdir -p results

docker run --rm \
  -v "$(pwd)/sample-data:/data:ro" \
  -v "$(pwd)/results:/results" \
  ad-aggregator /data/ad_data.csv /results/
```

This writes:
- `results/top10_ctr.csv`
- `results/top10_cpa.csv`

### 3. Run with your own CSV

Put your CSV inside any local folder, then mount that folder to `/data`:

```bash
mkdir -p results

docker run --rm \
  -v "/absolute/path/to/your/input-folder:/data:ro" \
  -v "$(pwd)/results:/results" \
  ad-aggregator /data/ad_data.csv /results/
```

If your file name is different, pass that path instead:

```bash
docker run --rm \
  -v "/absolute/path/to/your/input-folder:/data:ro" \
  -v "$(pwd)/results:/results" \
  ad-aggregator /data/my-file.csv /results/
```

### Docker Compose

The included compose file also works out of the box with the sample data:

```bash
docker compose up aggregator
```

To run containerized tests:

```bash
docker compose --profile test up test
```

## Local Go Run

Docker is the recommended path for reproducible execution. Local Go execution is also supported if Go `1.22+` is installed.

```bash
go run . sample-data/ad_data.csv
```

Custom output directory:

```bash
go run . sample-data/ad_data.csv results/
```

Flag-based form:

```bash
go run . --input sample-data/ad_data.csv --output results/
```

Build binary locally:

```bash
go build -o ad-aggregator .
./ad-aggregator sample-data/ad_data.csv
```

## Setup Instructions

### Prerequisites

- Docker, or
- Go `1.22+`

### Project Setup

```bash
git clone <your-repo>
cd analytics_csv
go mod tidy
```

## Input And Output

Input CSV schema:

```text
campaign_id,date,impressions,clicks,spend,conversions
```

Malformed rows are logged and skipped.

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
| `runtime` | Worker defaults and memory stats |
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

The program prints memory metrics on each run, including:
- `PeakHeapAlloc`
- `HeapAlloc`
- `TotalAlloc`
- `Sys`

Expected working range for the original 1GB benchmark target:
- around `180-250 MB` peak heap

Example benchmark-style log:

```text
2025/01/01 12:00:00 Starting: input=ad_data.csv workers=8 batch=8000
2025/01/01 12:00:10 Aggregated 1000 campaigns in 9.87s
Written: results/top10_ctr.csv (10 rows)
Written: results/top10_cpa.csv (10 rows)
2025/01/01 12:00:10 Memory - PeakHeapAlloc: 194.20 MB | HeapAlloc: 194.20 MB | TotalAlloc: 3518.44 MB | Sys: 320.00 MB
2025/01/01 12:00:10 Done! Total time: 10.02s
```

Example local run on the repository sample data:

```text
2026/03/18 21:27:55 Starting: input=ad_data.csv workers=10 batch=8000
2026/03/18 21:27:55 Aggregated 50 campaigns in 5.61s
2026/03/18 21:27:55 Written: results/top10_ctr.csv (10 rows)
2026/03/18 21:27:55 Written: results/top10_cpa.csv (10 rows)
2026/03/18 21:27:55 Memory - PeakHeapAlloc: 6.13 MB | HeapAlloc: 3.11 MB | TotalAlloc: 5334.68 MB | Sys: 16.64 MB
2026/03/18 21:27:55 Done! Total time: 5.61s
```

## Dockerfile

This repository includes a multi-stage [Dockerfile](/Users/dunglda/Documents/WORKING/analytics_csv/Dockerfile):
- `builder` builds the binary
- `tester` runs `go test -race -v ./tests/`
- final `scratch` image ships only the compiled binary

Container test command:

```bash
docker build --target tester -t ad-aggregator:test .
```

## Test Commands

Local:

```bash
go test -race -v ./tests/
```

Current test coverage includes:
- exact output validation on fixture data
- malformed row skipping
- extra-column rejection
- negative-value rejection
- missing input file handling
- deterministic tie-breaking
- race detection with `go test -race`

## AI Coding Assistants

This submission includes [PROMPTS.md](/Users/dunglda/Documents/WORKING/analytics_csv/PROMPTS.md) in the repository root with raw prompts used during development, as requested.
