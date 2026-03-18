package aggregator

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	defaultBufferSize = 4 * 1024 * 1024
	outputFileNameCTR = "top10_ctr.csv"
	outputFileNameCPA = "top10_cpa.csv"
)

var outputHeader = []string{
	"campaign_id",
	"total_impressions",
	"total_clicks",
	"total_spend",
	"total_conversions",
	"CTR",
	"CPA",
}

type Config struct {
	InputPath string
	OutputDir string
	Workers   int
	BatchSize int
}

type Row struct {
	CampaignID  string
	Date        string
	Impressions int64
	Clicks      int64
	Spend       float64
	Conversions int64
}

type Stats struct {
	TotalImpressions int64
	TotalClicks      int64
	TotalSpend       float64
	TotalConversions int64
}

type CampaignResult struct {
	CampaignID       string
	TotalImpressions int64
	TotalClicks      int64
	TotalSpend       float64
	TotalConversions int64
	CTR              float64
	CPA              *float64
}

type RunStats struct {
	Campaigns       int
	CTRRows         int
	CPARows         int
	SkippedRows     int
	CTROutputPath   string
	CPAOutputPath   string
	ProcessDuration time.Duration
	Memory          MemoryStats
}

type MemoryStats struct {
	PeakHeapAllocMB float64
	HeapAllocMB     float64
	TotalAllocMB    float64
	SysMB           float64
}

type parseStats struct {
	skippedRows int
}

func Run(cfg Config) (RunStats, error) {
	if cfg.Workers <= 0 {
		cfg.Workers = runtime.NumCPU()
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 8000
	}
	if cfg.OutputDir == "" {
		cfg.OutputDir = "results/"
	}

	if err := os.MkdirAll(cfg.OutputDir, 0o755); err != nil {
		return RunStats{}, fmt.Errorf("create output dir: %w", err)
	}

	start := time.Now()
	resetPeakHeap()
	stopMemoryMonitor := startMemoryMonitor()
	defer stopMemoryMonitor()

	campaignStats, parserStats, err := aggregateFile(cfg.InputPath, cfg.Workers, cfg.BatchSize)
	if err != nil {
		return RunStats{}, err
	}

	finalized := finalizeCampaigns(campaignStats)
	ctrTop := topByCTR(finalized, 10)
	cpaTop := topByCPA(finalized, 10)

	ctrPath := filepath.Join(cfg.OutputDir, outputFileNameCTR)
	cpaPath := filepath.Join(cfg.OutputDir, outputFileNameCPA)

	if err := writeResults(ctrPath, ctrTop); err != nil {
		return RunStats{}, err
	}
	if err := writeResults(cpaPath, cpaTop); err != nil {
		return RunStats{}, err
	}

	mem := currentMemoryStats()

	return RunStats{
		Campaigns:       len(finalized),
		CTRRows:         len(ctrTop),
		CPARows:         len(cpaTop),
		SkippedRows:     parserStats.skippedRows,
		CTROutputPath:   ctrPath,
		CPAOutputPath:   cpaPath,
		ProcessDuration: time.Since(start),
		Memory:          mem,
	}, nil
}

func ParseLine(line string) (Row, error) {
	parts := splitCSVLine(line)
	if len(parts) != 6 {
		return Row{}, fmt.Errorf("expected 6 columns, got %d", len(parts))
	}

	impressions, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		return Row{}, fmt.Errorf("invalid impressions: %w", err)
	}
	if impressions < 0 {
		return Row{}, fmt.Errorf("invalid impressions: must be non-negative")
	}
	clicks, err := strconv.ParseInt(parts[3], 10, 64)
	if err != nil {
		return Row{}, fmt.Errorf("invalid clicks: %w", err)
	}
	if clicks < 0 {
		return Row{}, fmt.Errorf("invalid clicks: must be non-negative")
	}
	spend, err := strconv.ParseFloat(parts[4], 64)
	if err != nil {
		return Row{}, fmt.Errorf("invalid spend: %w", err)
	}
	if spend < 0 {
		return Row{}, fmt.Errorf("invalid spend: must be non-negative")
	}
	conversions, err := strconv.ParseInt(parts[5], 10, 64)
	if err != nil {
		return Row{}, fmt.Errorf("invalid conversions: %w", err)
	}
	if conversions < 0 {
		return Row{}, fmt.Errorf("invalid conversions: must be non-negative")
	}

	return Row{
		CampaignID:  parts[0],
		Date:        parts[1],
		Impressions: impressions,
		Clicks:      clicks,
		Spend:       spend,
		Conversions: conversions,
	}, nil
}

func aggregateFile(path string, workers, batchSize int) (map[string]*Stats, parseStats, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, parseStats{}, fmt.Errorf("open input: %w", err)
	}
	defer file.Close()

	jobs := make(chan []Row, workers)
	results := make(chan map[string]*Stats, workers)
	mergedCh := make(chan map[string]*Stats, 1)
	parseCh := make(chan parseStats, 1)

	go func() {
		merged := make(map[string]*Stats)
		for partial := range results {
			mergeStatsMap(merged, partial)
		}
		mergedCh <- merged
	}()

	var workerWG sync.WaitGroup
	for i := 0; i < workers; i++ {
		workerWG.Add(1)
		go worker(jobs, results, &workerWG)
	}

	readerErrCh := make(chan error, 1)
	go func() {
		stats, err := readRows(file, batchSize, jobs)
		parseCh <- stats
		readerErrCh <- err
		close(jobs)
	}()

	workerWG.Wait()
	close(results)

	merged := <-mergedCh
	parserStats := <-parseCh
	if err := <-readerErrCh; err != nil {
		return nil, parserStats, err
	}

	return merged, parserStats, nil
}

func readRows(file *os.File, batchSize int, jobs chan<- []Row) (parseStats, error) {
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 1024*1024), defaultBufferSize)

	lineNumber := 0
	batch := make([]Row, 0, batchSize)
	stats := parseStats{}

	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if lineNumber == 1 && isHeaderLine(line) {
			continue
		}

		row, err := ParseLine(line)
		if err != nil {
			stats.skippedRows++
			log.Printf("Skipping malformed row at line %d: %v", lineNumber, err)
			continue
		}

		batch = append(batch, row)
		if len(batch) == batchSize {
			jobs <- batch
			batch = make([]Row, 0, batchSize)
		}
	}

	if err := scanner.Err(); err != nil {
		return stats, fmt.Errorf("scan input: %w", err)
	}

	if len(batch) > 0 {
		jobs <- batch
	}

	return stats, nil
}

func worker(jobs <-chan []Row, results chan<- map[string]*Stats, wg *sync.WaitGroup) {
	defer wg.Done()

	local := make(map[string]*Stats)
	for batch := range jobs {
		for _, row := range batch {
			stats := local[row.CampaignID]
			if stats == nil {
				stats = &Stats{}
				local[row.CampaignID] = stats
			}
			stats.TotalImpressions += row.Impressions
			stats.TotalClicks += row.Clicks
			stats.TotalSpend += row.Spend
			stats.TotalConversions += row.Conversions
		}
	}

	results <- local
}

func mergeStatsMap(dst map[string]*Stats, src map[string]*Stats) {
	for campaignID, partial := range src {
		current := dst[campaignID]
		if current == nil {
			current = &Stats{}
			dst[campaignID] = current
		}
		current.TotalImpressions += partial.TotalImpressions
		current.TotalClicks += partial.TotalClicks
		current.TotalSpend += partial.TotalSpend
		current.TotalConversions += partial.TotalConversions
	}
}

func finalizeCampaigns(campaigns map[string]*Stats) []CampaignResult {
	finalized := make([]CampaignResult, 0, len(campaigns))
	for campaignID, stats := range campaigns {
		result := CampaignResult{
			CampaignID:       campaignID,
			TotalImpressions: stats.TotalImpressions,
			TotalClicks:      stats.TotalClicks,
			TotalSpend:       stats.TotalSpend,
			TotalConversions: stats.TotalConversions,
		}
		if stats.TotalImpressions > 0 {
			result.CTR = float64(stats.TotalClicks) / float64(stats.TotalImpressions)
		}
		if stats.TotalConversions > 0 {
			cpa := stats.TotalSpend / float64(stats.TotalConversions)
			result.CPA = &cpa
		}
		finalized = append(finalized, result)
	}
	return finalized
}

func topByCTR(campaigns []CampaignResult, limit int) []CampaignResult {
	ordered := append([]CampaignResult(nil), campaigns...)
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].CTR == ordered[j].CTR {
			return ordered[i].CampaignID < ordered[j].CampaignID
		}
		return ordered[i].CTR > ordered[j].CTR
	})
	if len(ordered) > limit {
		ordered = ordered[:limit]
	}
	return ordered
}

func topByCPA(campaigns []CampaignResult, limit int) []CampaignResult {
	filtered := make([]CampaignResult, 0, len(campaigns))
	for _, campaign := range campaigns {
		if campaign.CPA != nil {
			filtered = append(filtered, campaign)
		}
	}
	sort.Slice(filtered, func(i, j int) bool {
		if *filtered[i].CPA == *filtered[j].CPA {
			return filtered[i].CampaignID < filtered[j].CampaignID
		}
		return *filtered[i].CPA < *filtered[j].CPA
	})
	if len(filtered) > limit {
		filtered = filtered[:limit]
	}
	return filtered
}

func writeResults(path string, rows []CampaignResult) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create output file %s: %w", path, err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	if err := writer.Write(outputHeader); err != nil {
		return fmt.Errorf("write header %s: %w", path, err)
	}

	for _, row := range rows {
		cpaValue := 0.0
		if row.CPA != nil {
			cpaValue = *row.CPA
		}

		record := []string{
			row.CampaignID,
			strconv.FormatInt(row.TotalImpressions, 10),
			strconv.FormatInt(row.TotalClicks, 10),
			fmt.Sprintf("%.4f", row.TotalSpend),
			strconv.FormatInt(row.TotalConversions, 10),
			fmt.Sprintf("%.6f", row.CTR),
			fmt.Sprintf("%.4f", cpaValue),
		}
		if err := writer.Write(record); err != nil {
			return fmt.Errorf("write row %s: %w", path, err)
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return fmt.Errorf("flush %s: %w", path, err)
	}
	return nil
}

func splitCSVLine(line string) []string {
	parts := strings.Split(line, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

func isHeaderLine(line string) bool {
	parts := splitCSVLine(line)
	if len(parts) < 6 {
		return false
	}
	first := strings.ToLower(parts[0])
	second := strings.ToLower(parts[1])
	return strings.Contains(first, "campaign") || strings.Contains(second, "date")
}

func currentMemoryStats() MemoryStats {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	return MemoryStats{
		PeakHeapAllocMB: peakHeapAllocMB(),
		HeapAllocMB:     float64(ms.HeapAlloc) / (1024 * 1024),
		TotalAllocMB:    float64(ms.TotalAlloc) / (1024 * 1024),
		SysMB:           float64(ms.Sys) / (1024 * 1024),
	}
}

var (
	peakHeapMu sync.Mutex
	peakHeap   uint64
)

func startMemoryMonitor() func() {
	done := make(chan struct{})
	ticker := time.NewTicker(10 * time.Millisecond)
	recordPeakHeap()

	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				recordPeakHeap()
			case <-done:
				recordPeakHeap()
				return
			}
		}
	}()

	return func() {
		close(done)
	}
}

func recordPeakHeap() {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)

	peakHeapMu.Lock()
	if ms.HeapAlloc > peakHeap {
		peakHeap = ms.HeapAlloc
	}
	peakHeapMu.Unlock()
}

func peakHeapAllocMB() float64 {
	peakHeapMu.Lock()
	defer peakHeapMu.Unlock()
	return float64(peakHeap) / (1024 * 1024)
}

func resetPeakHeap() {
	peakHeapMu.Lock()
	peakHeap = 0
	peakHeapMu.Unlock()
}
