package tests

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"project/pkg/aggregator"
)

func TestParseLineParsesExpectedRow(t *testing.T) {
	t.Parallel()

	row, err := aggregator.ParseLine("CMP001,2026-03-01,100,10,50.25,2")
	if err != nil {
		t.Fatalf("ParseLine() error = %v", err)
	}

	if row.CampaignID != "CMP001" || row.Date != "2026-03-01" {
		t.Fatalf("unexpected identity fields: %+v", row)
	}
	if row.Impressions != 100 || row.Clicks != 10 || row.Spend != 50.25 || row.Conversions != 2 {
		t.Fatalf("unexpected numeric fields: %+v", row)
	}
}

func TestParseLineRejectsExtraColumns(t *testing.T) {
	t.Parallel()

	_, err := aggregator.ParseLine("CMP001,2026-03-01,100,10,50.25,2,extra")
	if err == nil {
		t.Fatal("ParseLine() error = nil, want extra column error")
	}
}

func TestParseLineRejectsNegativeValues(t *testing.T) {
	t.Parallel()

	_, err := aggregator.ParseLine("CMP001,2026-03-01,-100,10,50.25,2")
	if err == nil {
		t.Fatal("ParseLine() error = nil, want negative value error")
	}
}

func TestParseLineHandlesThousandRows(t *testing.T) {
	t.Parallel()

	for i := 0; i < 1000; i++ {
		line := fmt.Sprintf("CMP%03d,2026-03-%02d,%d,%d,%.2f,%d", i%25, (i%28)+1, 100+i, 10+(i%7), 20.5+float64(i), 1+(i%3))
		row, err := aggregator.ParseLine(line)
		if err != nil {
			t.Fatalf("ParseLine() failed at row %d: %v", i, err)
		}
		if row.CampaignID == "" || row.Date == "" {
			t.Fatalf("parsed row missing identity fields at row %d: %+v", i, row)
		}
	}
}

func TestRunEndToEnd(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	input := fixturePath("sample_ad_data.csv")

	stats, err := aggregator.Run(aggregator.Config{
		InputPath: input,
		OutputDir: tmpDir,
		Workers:   3,
		BatchSize: 2,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if stats.Campaigns != 5 {
		t.Fatalf("Campaigns = %d, want 5", stats.Campaigns)
	}
	if stats.SkippedRows != 0 {
		t.Fatalf("SkippedRows = %d, want 0", stats.SkippedRows)
	}

	assertFileEquals(t, filepath.Join(tmpDir, "top10_ctr.csv"), strings.Join([]string{
		"campaign_id,total_impressions,total_clicks,total_spend,total_conversions,CTR,CPA",
		"CMP003,50,10,30.0000,5,0.200000,6.0000",
		"CMP001,150,15,75.0000,3,0.100000,25.0000",
		"CMP004,100,10,100.0000,10,0.100000,10.0000",
		"CMP002,200,10,50.0000,0,0.050000,0.0000",
		"CMP005,0,0,12.0000,1,0.000000,12.0000",
	}, "\n")+"\n")

	assertFileEquals(t, filepath.Join(tmpDir, "top10_cpa.csv"), strings.Join([]string{
		"campaign_id,total_impressions,total_clicks,total_spend,total_conversions,CTR,CPA",
		"CMP003,50,10,30.0000,5,0.200000,6.0000",
		"CMP004,100,10,100.0000,10,0.100000,10.0000",
		"CMP005,0,0,12.0000,1,0.000000,12.0000",
		"CMP001,150,15,75.0000,3,0.100000,25.0000",
	}, "\n")+"\n")
}

func TestRunCreatesOutputDir(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	outputDir := filepath.Join(base, "nested", "results")

	_, err := aggregator.Run(aggregator.Config{
		InputPath: fixturePath("sample_ad_data.csv"),
		OutputDir: outputDir,
		Workers:   2,
		BatchSize: 2,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if _, err := os.Stat(outputDir); err != nil {
		t.Fatalf("output dir not created: %v", err)
	}
}

func TestRunMissingInputFile(t *testing.T) {
	t.Parallel()

	_, err := aggregator.Run(aggregator.Config{
		InputPath: filepath.Join(t.TempDir(), "missing.csv"),
		OutputDir: t.TempDir(),
		Workers:   1,
		BatchSize: 1,
	})
	if err == nil {
		t.Fatal("Run() error = nil, want missing file error")
	}
}

func TestRunSkipsHeaderAndHandlesEmptyInput(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	input := filepath.Join(tmpDir, "empty.csv")
	if err := os.WriteFile(input, []byte("campaign_id,date,impressions,clicks,spend,conversions\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	stats, err := aggregator.Run(aggregator.Config{
		InputPath: input,
		OutputDir: tmpDir,
		Workers:   1,
		BatchSize: 1,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if stats.Campaigns != 0 || stats.CTRRows != 0 || stats.CPARows != 0 {
		t.Fatalf("unexpected stats: %+v", stats)
	}
}

func TestRunSkipsMalformedRows(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	input := filepath.Join(tmpDir, "bad.csv")
	content := strings.Join([]string{
		"campaign_id,date,impressions,clicks,spend,conversions",
		"CMP001,2026-03-01,100,10,12.50,1",
		"CMP002,2026-03-01,200,abc,30.00,2",
		"CMP002,2026-03-01,200,10,30.00,2,extra",
		"CMP002,2026-03-01,-200,10,30.00,2",
		"CMP003,2026-03-01,100,5,10.00,1",
	}, "\n")
	if err := os.WriteFile(input, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	stats, err := aggregator.Run(aggregator.Config{
		InputPath: input,
		OutputDir: tmpDir,
		Workers:   1,
		BatchSize: 1,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if stats.SkippedRows != 3 {
		t.Fatalf("SkippedRows = %d, want 3", stats.SkippedRows)
	}

	assertFileEquals(t, filepath.Join(tmpDir, "top10_ctr.csv"), strings.Join([]string{
		"campaign_id,total_impressions,total_clicks,total_spend,total_conversions,CTR,CPA",
		"CMP001,100,10,12.5000,1,0.100000,12.5000",
		"CMP003,100,5,10.0000,1,0.050000,10.0000",
	}, "\n")+"\n")
}

func TestRunDeterministicTieBreakers(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	input := filepath.Join(tmpDir, "ties.csv")
	content := strings.Join([]string{
		"campaign_id,date,impressions,clicks,spend,conversions",
		"CMP200,2026-03-01,100,10,50.0,5",
		"CMP100,2026-03-01,100,10,50.0,5",
		"CMP300,2026-03-01,100,10,50.0,5",
	}, "\n")
	if err := os.WriteFile(input, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, err := aggregator.Run(aggregator.Config{
		InputPath: input,
		OutputDir: tmpDir,
		Workers:   2,
		BatchSize: 2,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	assertFileEquals(t, filepath.Join(tmpDir, "top10_ctr.csv"), strings.Join([]string{
		"campaign_id,total_impressions,total_clicks,total_spend,total_conversions,CTR,CPA",
		"CMP100,100,10,50.0000,5,0.100000,10.0000",
		"CMP200,100,10,50.0000,5,0.100000,10.0000",
		"CMP300,100,10,50.0000,5,0.100000,10.0000",
	}, "\n")+"\n")
}

func assertFileEquals(t *testing.T, path string, want string) {
	t.Helper()

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", path, err)
	}
	if string(got) != want {
		t.Fatalf("file %s mismatch\nwant:\n%s\ngot:\n%s", path, want, string(got))
	}
}

func fixturePath(name string) string {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		return filepath.Join("testdata", name)
	}
	return filepath.Join(filepath.Dir(currentFile), "testdata", name)
}
