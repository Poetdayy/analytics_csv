# Ad Aggregator Implementation Notes

## Task 1 — Project scaffold & CLI

- Giữ module Go bằng `go.mod` để `go run .` và `go build` chạy trực tiếp từ root project.
- Dùng `flag` trong [main.go](/Users/dunglda/Documents/WORKING/analytics_csv/main.go) để parse `--input`, `--output`, `--workers`, `--batch`.
- Ngoài flags, CLI nhận positional args để gọi ngắn hơn:
  - `go run . ad_data.csv`
  - `go run . ad_data.csv results/`
- Nếu thiếu `--input`, chương trình in usage và exit code `2`.
- `Run()` tạo output directory ngay từ đầu bằng `os.MkdirAll`, nên không bị panic khi folder chưa tồn tại.

## Task 2 — CSV parser + Row struct

- Định nghĩa `Row` trong [pkg/aggregator/aggregator.go](/Users/dunglda/Documents/WORKING/analytics_csv/pkg/aggregator/aggregator.go) với các field:
  - `CampaignID`
  - `Date`
  - `Impressions`
  - `Clicks`
  - `Spend`
  - `Conversions`
- Dùng `bufio.Scanner` để đọc từng dòng, tăng buffer scanner lên 4MB để tránh choke với dòng lớn.
- Parse thủ công bằng `strings.Split` và `strconv.ParseInt` / `strconv.ParseFloat`.
- Header được bỏ qua nếu là dòng đầu và chứa `campaign` / `date`.
- Dòng lỗi không làm fail toàn bộ job nữa; code log lỗi rồi skip dòng đó.

## Task 3 — Worker pool

- Reader goroutine parse file thành `[]Row` theo batch size.
- Mỗi batch được đẩy vào `jobs chan`.
- Số worker mặc định là `runtime.NumCPU()`.
- Mỗi worker aggregate vào `map[string]*Stats` cục bộ, không dùng mutex trong hot path.
- Khi worker xong, worker push local map vào `results chan`.
- `sync.WaitGroup` được dùng để chờ toàn bộ worker kết thúc trước khi đóng `results`.

## Task 4 — Merger & final aggregation

- Có một merger goroutine riêng đọc `results chan`.
- Merger merge toàn bộ partial map vào một `map[string]*Stats` cuối cùng.
- Sau khi merge xong, chương trình tính:
  - `CTR = TotalClicks / TotalImpressions`
  - `CPA = TotalSpend / TotalConversions`
- Nếu `TotalConversions == 0` thì `CPA = nil` ở model nội bộ. Khi ghi file CSV, giá trị hiển thị là `0.0000`, còn danh sách top CPA thì loại campaign đó ra trước khi sort.

## Task 5 — Sort & write CSV output

- Chuyển map aggregate sang slice `[]CampaignResult`.
- `top10_ctr.csv`:
  - sort giảm dần theo `CTR`
  - nếu bằng nhau thì sort tăng dần theo `campaign_id`
- `top10_cpa.csv`:
  - filter bỏ campaign có `CPA == nil`
  - sort tăng dần theo `CPA`
  - nếu bằng nhau thì sort tăng dần theo `campaign_id`
- Ghi output bằng `encoding/csv` để tránh lỗi format khi export.

## Task 6 — Benchmark & README

- `Run()` và `main()` dùng `time.Now()` / `time.Since()` để log processing time.
- `runtime.ReadMemStats()` được dùng để log heap allocation và memory summary.
- README đã mô tả setup local, Docker, Docker Compose, test, output format và benchmark mẫu.

## Test coverage

- [tests/aggregator_test.go](/Users/dunglda/Documents/WORKING/analytics_csv/tests/aggregator_test.go) kiểm tra:
  - parse 1000 dòng sample thành công
  - end-to-end output đúng
  - output dir được tạo tự động
  - malformed row bị skip thay vì fail
  - tie-breaker deterministic
  - `go test -race` không có race condition
