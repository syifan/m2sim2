package benchmarks

import (
	"bytes"
	"testing"
)

// TestAccuracyCPI_WithDCache runs all microbenchmarks with D-cache enabled.
// This is the test used by accuracy_report.py to get CPI values that are
// comparable to real M2 hardware measurements (which include cache effects).
// Output format: "    benchmark_name: CPI=X.XXX" matching the parser in
// accuracy_report.py.
func TestAccuracyCPI_WithDCache(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	config := DefaultConfig()
	config.Output = &bytes.Buffer{}
	config.EnableICache = false
	config.EnableDCache = true

	harness := NewHarness(config)
	harness.AddBenchmarks(GetMicrobenchmarks())

	results := harness.RunAll()

	for _, r := range results {
		t.Logf("    %s: CPI=%.3f", r.Name, r.CPI)
	}
}
