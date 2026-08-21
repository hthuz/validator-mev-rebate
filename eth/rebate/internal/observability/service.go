package observability

import (
	"rebate/internal/logging"
	"rebate/pkg/types"

	"github.com/ethereum/go-ethereum/common"
)

// Service is the single observability entry point used by business modules.
type Service struct {
	store    *MetricsStore
	recorder *ExperimentRecorder
}

func NewService(experimentDir string) (*Service, error) {
	recorder, err := NewExperimentRecorder(experimentDir)
	if err != nil {
		return nil, err
	}
	return &Service{
		store:    NewMetricsStore(),
		recorder: recorder,
	}, nil
}

func (s *Service) Store() *MetricsStore {
	if s == nil {
		return nil
	}
	return s.store
}

func (s *Service) BaseDir() string {
	if s == nil || s.recorder == nil {
		return ""
	}
	return s.recorder.BaseDir()
}

func (s *Service) Close() error {
	if s == nil || s.recorder == nil {
		return nil
	}
	return s.recorder.Close()
}

func (s *Service) WriteMetadata(value any) error {
	if s == nil || s.recorder == nil {
		return nil
	}
	return s.recorder.WriteMetadata(value)
}

func (s *Service) StartNewBlock(blockNumber uint64, validator common.Address) *BlockMevMetrics {
	if s == nil || s.store == nil {
		return nil
	}
	return s.store.StartNewBlock(blockNumber, validator)
}

func (s *Service) FinalizeBlock(blockNumber uint64, blockGasLimit uint64) {
	if s == nil || s.store == nil {
		return
	}
	event, ok := s.store.FinalizeBlock(blockNumber, blockGasLimit)
	if !ok || s.recorder == nil || event == nil {
		return
	}
	if err := s.recorder.RecordBlockSummary(*event); err != nil {
		logging.Logger.Warn().Err(err).Uint64("blockNumber", event.BlockNumber).Msg("Failed to record block summary")
	}
}

func (s *Service) RecordBundleResult(blockNumber uint64, result *types.SimMevBundleResponse, builder string, searcher common.Address) {
	if s == nil || s.store == nil {
		return
	}
	s.store.RecordBundleResult(blockNumber, result, builder, searcher)
}

func (s *Service) RecordBundleSimulation(event BundleSimulationEvent) {
	if s == nil || s.recorder == nil {
		return
	}
	if err := s.recorder.RecordBundleSimulation(event); err != nil {
		logging.Logger.Warn().Err(err).Msg("Failed to record bundle simulation event")
	}
}

func (s *Service) RecordDispatch(event BuilderDispatchEvent) {
	if s == nil || s.recorder == nil {
		return
	}
	if err := s.recorder.RecordBuilderDispatch(event); err != nil {
		logging.Logger.Warn().Err(err).Msg("Failed to record builder dispatch event")
	}
}

func (s *Service) RecordBuilderDispatch(event BuilderDispatchEvent) {
	s.RecordDispatch(event)
}

func (s *Service) RecordBuilderSnapshot(event BuilderSnapshotEvent) {
	if s == nil || s.recorder == nil {
		return
	}
	if err := s.recorder.RecordBuilderSnapshot(event); err != nil {
		logging.Logger.Warn().Err(err).Msg("Failed to record builder snapshot")
	}
}
