package experiment

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRecorderWritesJSONLFiles(t *testing.T) {
	dir := t.TempDir()
	recorder, err := NewRecorder(dir)
	if err != nil {
		t.Fatalf("NewRecorder: %v", err)
	}
	defer recorder.Close()

	now := time.Unix(1710000000, 0).UTC()
	if err := recorder.RecordBundleSimulation(BundleSimulationEvent{
		RecordedAt:        now,
		BundleHash:        "0xabc",
		TargetBlock:       123,
		SimulationSuccess: true,
		ProfitWei:         "100",
		RefundableWei:     "10",
		MevGasPriceWei:    "2",
	}); err != nil {
		t.Fatalf("RecordBundleSimulation: %v", err)
	}
	if err := recorder.RecordBuilderDispatch(BuilderDispatchEvent{
		RecordedAt:      now,
		BundleHash:      "0xabc",
		TargetBlock:     123,
		Builder:         "builder-a",
		Layer:           "exploration",
		Reason:          "ucb",
		Success:         true,
		TotalScore:      4,
		BundleProfitWei: "100",
	}); err != nil {
		t.Fatalf("RecordBuilderDispatch: %v", err)
	}
	if err := recorder.RecordBuilderSnapshot(BuilderSnapshotEvent{
		RecordedAt:     now,
		Source:         "dispatch_observation",
		Builder:        "builder-a",
		BaseScore:      1,
		EffectiveScore: 1.5,
	}); err != nil {
		t.Fatalf("RecordBuilderSnapshot: %v", err)
	}
	if err := recorder.RecordBlockSummary(BlockSummaryEvent{
		RecordedAt:         now,
		BlockNumber:        123,
		Validator:          "0x1",
		BlockTimestamp:     now,
		BundleCount:        2,
		SuccessCount:       1,
		FailedCount:        1,
		TotalMevProfitWei:  "100",
		TotalRefundableWei: "10",
		MevGasPriceWei:     "2",
		BuilderDistribution: map[string]int{
			"builder-a": 2,
		},
	}); err != nil {
		t.Fatalf("RecordBlockSummary: %v", err)
	}

	for _, name := range []string{
		bundleEventsFile,
		builderDispatchFile,
		builderSnapshotFile,
		blockSummaryFile,
	} {
		path := filepath.Join(dir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile(%s): %v", name, err)
		}
		if !strings.Contains(string(data), "\n") {
			t.Fatalf("expected newline-delimited json in %s", name)
		}
	}
}
