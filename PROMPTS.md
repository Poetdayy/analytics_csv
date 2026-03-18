# PROMPTS
PLEASE IMPLEMENT THIS PLAN:
# Build Out the Go Ad Aggregator Project

## Summary
Implement the missing Go application described by `README.md` so the repo can actually build and run in all three advertised modes: local binary, `docker build`/`docker run`, and `docker compose`. The repo currently contains only build/docs files; there is no `go.mod`, source package, or tests, so the work is effectively a full scaffold from the README spec.

## Key Changes
- Create the Go module and CLI entrypoint with flags `--input`, `--output`, `--workers`, and `--batch`, matching the README defaults and required/optional behavior.
- Implement the CSV aggregation pipeline as documented:
  - buffered file reader with batched row dispatch
  - worker pool with per-worker local maps
  - single-threaded merge of worker results
  - final CTR and CPA computation
  - write `top10_ctr.csv` and `top10_cpa.csv` with the documented column order and numeric formatting
- Define the internal data model around campaign aggregates: impressions, clicks, spend, conversions, derived CTR, derived CPA.
- Add robust row parsing and validation for malformed lines, header handling, zero-conversion CPA exclusion, and deterministic top-10 sorting behavior.
- Add tests under `./tests` to satisfy the advertised `go test -race -v ./tests/` flow.
- Align container/build assets with the real codebase:
  - add `go.mod` and `go.sum`
  - keep the multi-stage Docker build
  - ensure `docker compose up aggregator` runs the built binary against mounted input/output paths

## Public Interfaces
- Binary name: `ad-aggregator`
- CLI flags:
  - `--input` required CSV path
  - `--output` default `results/`
  - `--workers` default `runtime.NumCPU()`
  - `--batch` default `8000`
- Input contract:
  - CSV with a fixed ad-event schema sufficient to aggregate by `campaign_id`
  - first row treated as header if present
- Output files:
  - `results/top10_ctr.csv`
  - `results/top10_cpa.csv`
- Output columns:
  - `campaign_id,total_impressions,total_clicks,total_spend,total_conversions,CTR,CPA`

## Test Plan
- Unit tests for row parsing, numeric conversion, aggregation merge logic, CTR/CPA calculation, and sort/top-10 selection.
- End-to-end test with a small fixture CSV verifying both output files and exact formatting.
- Edge-case tests for:
  - zero impressions
  - zero conversions
  - malformed rows
  - empty input after header
  - ties in ranking
- Build verification targets:
  - `go build -o ad-aggregator .`
  - `go test -race -v ./tests/`
  - `docker build -t ad-aggregator .`
  - `docker build --target tester -t ad-aggregator:test .`
  - `docker compose --profile test up test`

## Assumptions
- The README is the source of truth for v1; no external code needs to be integrated.
- A sample `ad_data.csv` fixture will be added for tests/examples, but the large benchmark dataset is not required for initial implementation.
- Linux `amd64` remains the Docker target as currently declared in the `Dockerfile`, while local builds use the host platform.
- If the README does not define tie-breaking precisely, ranking should be deterministic by metric first, then `campaign_id` ascending.

---

Task 1 — Project scaffold & CLI
Việc cần làm:

go mod init project
Dùng flag hoặc cobra parse --input, --output
In usage khi thiếu args
Tạo output dir nếu chưa có

Acceptance: go run . --input foo.csv --output results/ chạy không panic, tạo được folder

Task 2 — CSV parser + Row struct
Việc cần làm:

Định nghĩa struct Row { CampaignID string; Date string; Impressions int64; Clicks int64; Spend float64; Conversions int64 }
Dùng bufio.Scanner đọc từng dòng, parse thủ công (split by comma) — không dùng encoding/csv vì chậm hơn với file lớn
Bỏ qua header line
Handle parse error gracefully (log và skip dòng lỗi)

Acceptance: parse được 1000 dòng sample, struct đúng giá trị

Task 3 — Worker pool (goroutines + channels)
Việc cần làm:

Reader goroutine đọc file, batch N dòng thành []Row, đẩy vào jobs chan
Spawn runtime.NumCPU() worker goroutines, mỗi worker nhận batch từ jobs chan
Mỗi worker tự aggregate vào local map[string]*Stats
Sau khi done, push local map vào results chan
Dùng sync.WaitGroup để biết khi nào tất cả workers xong

Struct Stats:
gotype Stats struct {
    TotalImpressions int64
    TotalClicks      int64
    TotalSpend       float64
    TotalConversions int64
}
Acceptance: chạy với file nhỏ, không race condition (go test -race)

Task 4 — Merger & final aggregation
Việc cần làm:

Merger goroutine đọc từ results chan, merge tất cả partial maps vào một global map[string]*Stats
Sau khi merge xong, tính CTR và CPA cho từng campaign
CPA = nil nếu TotalConversions == 0

Acceptance: kết quả đúng với test data nhỏ có expected output

Task 5 — Sort & write CSV output
Việc cần làm:

Convert map sang slice để sort
Top 10 CTR: sort descending by CTR, lấy 10 đầu
Top 10 CPA: filter bỏ nil CPA, sort ascending by CPA, lấy 10 đầu
Dùng encoding/csv để write output đúng format
Column order đúng với spec: campaign_id, total_impressions, total_clicks, total_spend, total_conversions, CTR, CPA

Acceptance: 2 file CSV output đúng format, giá trị chính xác

Task 6 — Benchmark & README
Việc cần làm:

Đo thời gian chạy bằng time.Now() / time.Since()
Đo peak memory bằng runtime.ReadMemStats
Log ra: processing time, peak heap alloc
Viết README với setup, run instructions, kết quả benchmark

Acceptance: README đầy đủ, có số liệu cụ thể

Một vài Go-specific tips quan trọng
Batch size: đừng push từng Row vào channel — overhead quá lớn. Batch 5000–10000 dòng mỗi lần là sweet spot.
Tránh global mutex: mỗi worker giữ local map riêng, chỉ merge 1 lần ở cuối. Đây là lý do tại sao architecture trên không cần sync.Mutex trong hot path.
String interning cho campaign_id: nếu muốn tối ưu thêm, campaign_id lặp đi lặp lại hàng triệu lần — có thể dùng một sync.Map nhỏ để intern string, giảm GC pressure.
Parse số thủ công: strconv.ParseInt và strconv.ParseFloat đã khá nhanh, dùng trực tiếp thay vì reflect-based CSV decoder.

Tôi cần triển khai các bước này để project tôi chạy được, hãy triển khai thành từng mục nhỏ và phân tích kỹ bạn làm như nào cho tôi, tốt nhất là hãy làm từng mục,  sau đó test cho tôi đảm bảo input cho ra đúng output bài toán.




