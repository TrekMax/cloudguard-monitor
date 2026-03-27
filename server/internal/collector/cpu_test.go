package collector

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

const testProcStat = `cpu  10132153 290696 3084719 46828483 16683 0 25195 0 0 0
cpu0 1393280 32966 572056 13343292 6130 0 17875 0 0 0
cpu1 1335428 28612 384152 13287738 4210 0 3668 0 0 0
`

const testProcStat2 = `cpu  10142153 290696 3094719 46838483 16783 0 25295 0 0 0
cpu0 1403280 32966 577056 13348292 6180 0 17925 0 0 0
cpu1 1345428 28612 389152 13292738 4260 0 3718 0 0 0
`

func writeTempProcStat(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "stat")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCPUCollector_Name(t *testing.T) {
	c := NewCPUCollector()
	if c.Name() != "cpu" {
		t.Errorf("expected name 'cpu', got %q", c.Name())
	}
}

func TestCPUCollector_FirstCollect(t *testing.T) {
	path := writeTempProcStat(t, testProcStat)
	c := NewCPUCollector()
	c.procStat = path

	metrics, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(metrics) != 1 {
		t.Fatalf("expected 1 metric, got %d", len(metrics))
	}
	// First collection returns 0 usage (no previous data)
	if metrics[0].Values["usage"] != 0 {
		t.Errorf("expected 0 usage on first collect, got %f", metrics[0].Values["usage"])
	}
	if metrics[0].Values["cores"] != 2 {
		t.Errorf("expected 2 cores, got %f", metrics[0].Values["cores"])
	}
}

func TestCPUCollector_SecondCollect(t *testing.T) {
	path := writeTempProcStat(t, testProcStat)
	c := NewCPUCollector()
	c.procStat = path

	// First collect to set baseline
	_, err := c.Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	// Update proc stat with new values
	if err := os.WriteFile(path, []byte(testProcStat2), 0644); err != nil {
		t.Fatal(err)
	}

	// Second collect should compute deltas
	metrics, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(metrics) != 1 {
		t.Fatalf("expected 1 metric, got %d", len(metrics))
	}

	usage := metrics[0].Values["usage"]
	if usage < 0 || usage > 100 {
		t.Errorf("usage out of range [0,100]: %f", usage)
	}
	// With the test data, usage should be positive and reasonable
	if usage < 10 || usage > 90 {
		t.Errorf("expected usage between 10%% and 90%%, got %f%%", usage)
	}
}

func TestParseCPULine(t *testing.T) {
	tests := []struct {
		name    string
		line    string
		wantErr bool
	}{
		{"valid", "cpu  1000 200 300 4000 50 0 60 0 0 0", false},
		{"short", "cpu  1000", true},
		{"invalid_number", "cpu  abc 200 300 4000 50 0 60 0", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseCPULine(tt.line)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseCPULine() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCPUCollector_InvalidPath(t *testing.T) {
	c := NewCPUCollector()
	c.procStat = "/nonexistent/path"

	_, err := c.Collect(context.Background())
	if err == nil {
		t.Error("expected error for invalid proc path")
	}
}
