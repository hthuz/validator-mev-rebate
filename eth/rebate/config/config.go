package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

// ServerConfig HTTP 服务器配置
type ServerConfig struct {
	Port int `mapstructure:"port"`
}

// BuilderConfig 单个 builder 节点配置
type BuilderConfig struct {
	Name  string  `mapstructure:"name"`
	URL   string  `mapstructure:"url"`
	Score float64 `mapstructure:"score"`
}

type StrategyConfig struct {
	ExplorationEnabled     bool          `mapstructure:"exploration_enabled"`
	ExplorationMode        string        `mapstructure:"exploration_mode"`
	ExplorationRate        float64       `mapstructure:"exploration_rate"`
	MinExploreDispatches   uint64        `mapstructure:"min_explore_dispatches"`
	NewProducerGracePeriod time.Duration `mapstructure:"new_producer_grace_period"`
	UncertaintyWeight      float64       `mapstructure:"uncertainty_weight"`
	FreshProducerBonus     float64       `mapstructure:"fresh_producer_bonus"`
}

type ReportingConfig struct {
	ExperimentDir string `mapstructure:"experiment_dir"`
}

// DispatcherConfig 分发器配置
type DispatcherConfig struct {
	Builders []BuilderConfig `mapstructure:"builders"`
	Strategy StrategyConfig  `mapstructure:"strategy"`
}

// SimulatorConfig 模拟器配置
type SimulatorConfig struct {
	Mode                 string `mapstructure:"mode"`
	DatasetPath          string `mapstructure:"dataset_path"`
	BlockIntervalSeconds int    `mapstructure:"block_interval_seconds"`
	BlockIntervalMillis  int    `mapstructure:"block_interval_milliseconds"`
	BlockGasLimit        uint64 `mapstructure:"block_gas_limit"`
	Workers              int    `mapstructure:"workers"`
}

// MockBuilderConfig mock builder 监听配置
type MockBuilderConfig struct {
	Name string `mapstructure:"name"`
	Addr string `mapstructure:"addr"`
}

// Config 全局配置
type Config struct {
	Server       ServerConfig        `mapstructure:"server"`
	Simulator    SimulatorConfig     `mapstructure:"simulator"`
	Dispatcher   DispatcherConfig    `mapstructure:"dispatcher"`
	Reporting    ReportingConfig     `mapstructure:"reporting"`
	MockBuilders []MockBuilderConfig `mapstructure:"mock_builders"`
}

// Load 从指定路径加载配置文件，path 为空时使用默认路径 config/config.yaml
func Load(path string) (*Config, error) {
	v := viper.New()

	if path != "" {
		v.SetConfigFile(path)
	} else {
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath("config")
		v.AddConfigPath(".")
	}

	// 环境变量覆盖，前缀 REBATE_
	v.SetEnvPrefix("REBATE")
	v.AutomaticEnv()

	setDefaults(v)

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	if err := validate(&cfg); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &cfg, nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("server.port", 8080)
	v.SetDefault("simulator.mode", "replay")
	v.SetDefault("simulator.dataset_path", "data/ethereum_transactions.csv")
	v.SetDefault("simulator.block_interval_seconds", 2)
	v.SetDefault("simulator.block_interval_milliseconds", 0)
	v.SetDefault("simulator.block_gas_limit", 30000000)
	v.SetDefault("simulator.workers", 1)
	v.SetDefault("dispatcher.strategy.exploration_enabled", true)
	v.SetDefault("dispatcher.strategy.exploration_mode", "probabilistic")
	v.SetDefault("dispatcher.strategy.exploration_rate", 0.20)
	v.SetDefault("dispatcher.strategy.min_explore_dispatches", 5)
	v.SetDefault("dispatcher.strategy.new_producer_grace_period", 600*time.Second)
	v.SetDefault("dispatcher.strategy.uncertainty_weight", 1.25)
	v.SetDefault("dispatcher.strategy.fresh_producer_bonus", 0.75)
	v.SetDefault("reporting.experiment_dir", "logs/experiment")
}

func validate(cfg *Config) error {
	if cfg.Server.Port <= 0 || cfg.Server.Port > 65535 {
		return fmt.Errorf("server.port %d is out of range", cfg.Server.Port)
	}
	if cfg.Simulator.Mode != "mock" && cfg.Simulator.Mode != "replay" {
		return fmt.Errorf("simulator.mode %q is invalid", cfg.Simulator.Mode)
	}
	if cfg.Simulator.BlockIntervalSeconds <= 0 {
		return fmt.Errorf("simulator.block_interval_seconds must be > 0")
	}
	if cfg.Simulator.BlockIntervalMillis < 0 {
		return fmt.Errorf("simulator.block_interval_milliseconds must be >= 0")
	}
	if cfg.Simulator.BlockGasLimit == 0 {
		return fmt.Errorf("simulator.block_gas_limit must be > 0")
	}
	if cfg.Simulator.Workers <= 0 {
		return fmt.Errorf("simulator.workers must be > 0")
	}
	if cfg.Simulator.Mode == "replay" && cfg.Simulator.DatasetPath == "" {
		return fmt.Errorf("simulator.dataset_path is required in replay mode")
	}
	if cfg.Reporting.ExperimentDir == "" {
		return fmt.Errorf("reporting.experiment_dir is required")
	}
	for i, b := range cfg.Dispatcher.Builders {
		if b.Name == "" {
			return fmt.Errorf("dispatcher.builders[%d]: name is required", i)
		}
		if b.URL == "" {
			return fmt.Errorf("dispatcher.builders[%d] %q: url is required", i, b.Name)
		}
		if b.Score <= 0 || b.Score > 100 {
			return fmt.Errorf("dispatcher.builders[%d] %q: score must be in (0, 100]", i, b.Name)
		}
	}
	if cfg.Dispatcher.Strategy.ExplorationRate < 0 || cfg.Dispatcher.Strategy.ExplorationRate > 1 {
		return fmt.Errorf("dispatcher.strategy.exploration_rate must be in [0,1]")
	}
	if cfg.Dispatcher.Strategy.ExplorationMode != "probabilistic" && cfg.Dispatcher.Strategy.ExplorationMode != "alternating" {
		return fmt.Errorf("dispatcher.strategy.exploration_mode must be probabilistic or alternating")
	}
	if cfg.Dispatcher.Strategy.NewProducerGracePeriod < 0 {
		return fmt.Errorf("dispatcher.strategy.new_producer_grace_period must be >= 0")
	}
	if cfg.Dispatcher.Strategy.UncertaintyWeight < 0 {
		return fmt.Errorf("dispatcher.strategy.uncertainty_weight must be >= 0")
	}
	if cfg.Dispatcher.Strategy.FreshProducerBonus < 0 {
		return fmt.Errorf("dispatcher.strategy.fresh_producer_bonus must be >= 0")
	}
	for i, m := range cfg.MockBuilders {
		if m.Addr == "" {
			return fmt.Errorf("mock_builders[%d]: addr is required", i)
		}
	}
	return nil
}
