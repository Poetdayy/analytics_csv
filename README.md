# Ad Performance Aggregator

A concurrent Go CLI for aggregating large ad-performance CSV files into:
- `top10_ctr.csv`
- `top10_cpa.csv`

The implementation uses a streaming reader, batched worker pool, local per-worker maps, and a final merger so it can process a large CSV in a single pass without loading the full file into memory.

## Quick Start With Docker

This repository is designed so another machine can run it immediately with Docker.

### Simplest flow

1. Clone the repository
2. Put your input file at the project root with this exact name:

```text
ad_data.csv
```

3. Run:

```bash
mkdir -p results
docker compose up aggregator
```

That is the main recommended path.

Results will be written to:
- `results/top10_ctr.csv`
- `results/top10_cpa.csv`

### Why this is the simplest option

- no Go installation required
- no long `docker run` command required
- user only needs to place `ad_data.csv` in the repo root
- `docker-compose.yml` already mounts the input and output paths correctly

### Read a file from anywhere, for example `Downloads`

You do not need to move the CSV into this repository.

Example:

```bash
mkdir -p results

INPUT_DIR="$HOME/Downloads" \
INPUT_FILE="ad_data.csv" \
docker compose up aggregator
```

If your file has a different name:

```bash
mkdir -p results

INPUT_DIR="$HOME/Downloads" \
INPUT_FILE="my-big-file.csv" \
docker compose up aggregator
```

If you want output somewhere else too:

```bash
INPUT_DIR="$HOME/Downloads" \
INPUT_FILE="my-big-file.csv" \
OUTPUT_DIR="$HOME/Desktop/ad-results" \
docker compose up aggregator
```

## Docker Commands

### Build manually

```bash
docker build -t ad-aggregator .
```

### Run manually

If `ad_data.csv` is already in the project root:

```bash
mkdir -p results

docker run --rm \
  -v "$(pwd):/workspace:ro" \
  -v "$(pwd)/results:/results" \
  ad-aggregator
```

The image now defaults to:

```text
--input=/workspace/ad_data.csv --output=/results/
```

So users do not need to pass those flags manually.

If the CSV is outside the repo, mount its parent directory:

```bash
docker run --rm \
  -v "$HOME/Downloads:/input:ro" \
  -v "$(pwd)/results:/results" \
  ad-aggregator --input /input/ad_data.csv --output /results/
```

### Run tests in Docker

```bash
docker build --target tester -t ad-aggregator:test .
```

Or:

```bash
docker compose --profile test up test
```

## Local Go Run

Docker is the recommended path for reproducible execution. Local Go execution is also supported if Go `1.22+` is installed.

```bash
go run . ad_data.csv
```

Custom output directory:

```bash
go run . ad_data.csv results/
```

Flag-based form:

```bash
go run . --input ad_data.csv --output results/
```

Build binary locally:

```bash
go build -o ad-aggregator .
./ad-aggregator ad_data.csv
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

## Dockerfile

This repository includes a multi-stage [Dockerfile](/Users/dunglda/Documents/WORKING/analytics_csv/Dockerfile):
- `builder` builds the binary
- `tester` runs `go test -race -v ./tests/`
- final `scratch` image ships only the compiled binary

## Test Commands

Local:

```bash
go test -race -v ./tests/
```

Containerized:

```bash
docker compose --profile test up test
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
