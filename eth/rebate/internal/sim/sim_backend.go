package sim

import (
	"context"
	"rebate/pkg/types"
)

// SimulationBackend 模拟后端接口
type SimulationBackend interface {
	SimulateBundle(ctx context.Context, bundle *types.SendMevBundleArgs, overrides map[string]interface{}) (*types.SimMevBundleResponse, error)
}
