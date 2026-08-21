package observability

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"rebate/internal/logging"
	"strings"
	"sync"
)

const defaultDir = "logs/experiment"

const (
	bundleEventsFile    = "bundle_events.jsonl"
	builderDispatchFile = "builder_dispatches.jsonl"
	builderSnapshotFile = "builder_snapshots.jsonl"
	blockSummaryFile    = "block_summary.jsonl"
	metadataFile        = "metadata.json"
)

type ExperimentRecorder struct {
	baseDir string
	mu      sync.Mutex
	files   map[string]*os.File
}

func DefaultDir() string {
	if dir := strings.TrimSpace(os.Getenv("EXPERIMENT_REPORT_DIR")); dir != "" {
		return logging.ResolveLogPath(dir)
	}
	return logging.ResolveLogPath(defaultDir)
}

func NewExperimentRecorder(baseDir string) (*ExperimentRecorder, error) {
	if strings.TrimSpace(baseDir) == "" {
		baseDir = DefaultDir()
	} else {
		baseDir = logging.ResolveLogPath(baseDir)
	}
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return nil, fmt.Errorf("create experiment dir: %w", err)
	}

	return &ExperimentRecorder{
		baseDir: baseDir,
		files:   make(map[string]*os.File),
	}, nil
}

func (r *ExperimentRecorder) BaseDir() string {
	if r == nil {
		return ""
	}
	return r.baseDir
}

func (r *ExperimentRecorder) WriteMetadata(value any) error {
	if r == nil {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("encode metadata: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(filepath.Join(r.baseDir, metadataFile), data, 0o644); err != nil {
		return fmt.Errorf("write metadata: %w", err)
	}
	return nil
}

func (r *ExperimentRecorder) Close() error {
	if r == nil {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	var errs []string
	for name, file := range r.files {
		if err := file.Close(); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", name, err))
		}
	}
	r.files = make(map[string]*os.File)
	if len(errs) > 0 {
		return fmt.Errorf("close experiment files: %s", strings.Join(errs, "; "))
	}
	return nil
}

func (r *ExperimentRecorder) RecordBundleSimulation(event BundleSimulationEvent) error {
	return r.appendJSONL(bundleEventsFile, event)
}

func (r *ExperimentRecorder) RecordBuilderDispatch(event BuilderDispatchEvent) error {
	return r.appendJSONL(builderDispatchFile, event)
}

func (r *ExperimentRecorder) RecordBuilderSnapshot(event BuilderSnapshotEvent) error {
	return r.appendJSONL(builderSnapshotFile, event)
}

func (r *ExperimentRecorder) RecordBlockSummary(event BlockSummaryEvent) error {
	return r.appendJSONL(blockSummaryFile, event)
}

func (r *ExperimentRecorder) appendJSONL(name string, value any) error {
	if r == nil {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	file, err := r.file(name)
	if err != nil {
		return err
	}

	enc := json.NewEncoder(file)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(value); err != nil {
		return fmt.Errorf("encode %s: %w", name, err)
	}
	return nil
}

func (r *ExperimentRecorder) file(name string) (*os.File, error) {
	if file, ok := r.files[name]; ok {
		return file, nil
	}

	path := filepath.Join(r.baseDir, name)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	r.files[name] = file
	return file, nil
}
