package config

import (
	"fmt"

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

// DispatcherConfig 分发器配置
type DispatcherConfig struct {
	Builders []BuilderConfig `mapstructure:"builders"`
}

// SimulatorConfig 模拟器配置
type SimulatorConfig struct {
	Mode                 string `mapstructure:"mode"`
	DatasetPath          string `mapstructure:"dataset_path"`
	BlockIntervalSeconds int    `mapstructure:"block_interval_seconds"`
	BlockGasLimit        uint64 `mapstructure:"block_gas_limit"`
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
	v.SetDefault("simulator.block_gas_limit", 30000000)
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
	if cfg.Simulator.BlockGasLimit == 0 {
		return fmt.Errorf("simulator.block_gas_limit must be > 0")
	}
	if cfg.Simulator.Mode == "replay" && cfg.Simulator.DatasetPath == "" {
		return fmt.Errorf("simulator.dataset_path is required in replay mode")
	}
	for i, b := range cfg.Dispatcher.Builders {
		if b.Name == "" {
			return fmt.Errorf("dispatcher.builders[%d]: name is required", i)
		}
		if b.URL == "" {
			return fmt.Errorf("dispatcher.builders[%d] %q: url is required", i, b.Name)
		}
		if b.Score <= 0 {
			return fmt.Errorf("dispatcher.builders[%d] %q: score must be > 0", i, b.Name)
		}
	}
	for i, m := range cfg.MockBuilders {
		if m.Addr == "" {
			return fmt.Errorf("mock_builders[%d]: addr is required", i)
		}
	}
	return nil
}
