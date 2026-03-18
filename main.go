package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
	"time"

	"project/pkg/aggregator"
)

func main() {
	cfg := aggregator.Config{}

	flag.StringVar(&cfg.InputPath, "input", "", "path to input CSV file")
	flag.StringVar(&cfg.OutputDir, "output", "results/", "directory for output CSV files")
	flag.IntVar(&cfg.Workers, "workers", runtime.NumCPU(), "number of worker goroutines")
	flag.IntVar(&cfg.BatchSize, "batch", 8000, "number of rows per batch")
	flag.Parse()

	args := flag.Args()
	if cfg.InputPath == "" && len(args) > 0 {
		cfg.InputPath = args[0]
	}
	if cfg.OutputDir == "results/" && len(args) > 1 {
		cfg.OutputDir = args[1]
	}

	if cfg.InputPath == "" {
		fmt.Fprintln(os.Stderr, "error: input CSV path is required")
		flag.Usage()
		fmt.Fprintln(os.Stderr, "\nExamples:")
		fmt.Fprintln(os.Stderr, "  go run . ad_data.csv")
		fmt.Fprintln(os.Stderr, "  go run . ad_data.csv results/")
		fmt.Fprintln(os.Stderr, "  go run . --input ad_data.csv --output results/")
		os.Exit(2)
	}

	start := time.Now()
	log.Printf("Starting: input=%s workers=%d batch=%d", cfg.InputPath, cfg.Workers, cfg.BatchSize)

	stats, err := aggregator.Run(cfg)
	if err != nil {
		log.Printf("Failed: %v", err)
		os.Exit(1)
	}

	log.Printf("Aggregated %d campaigns in %.2fs", stats.Campaigns, stats.ProcessDuration.Seconds())
	log.Printf("Written: %s (%d rows)", stats.CTROutputPath, stats.CTRRows)
	log.Printf("Written: %s (%d rows)", stats.CPAOutputPath, stats.CPARows)
	log.Printf(
		"Memory - PeakHeapAlloc: %.2f MB | HeapAlloc: %.2f MB | TotalAlloc: %.2f MB | Sys: %.2f MB",
		stats.Memory.PeakHeapAllocMB,
		stats.Memory.HeapAllocMB,
		stats.Memory.TotalAllocMB,
		stats.Memory.SysMB,
	)
	log.Printf("Done! Total time: %.2fs", time.Since(start).Seconds())
}
