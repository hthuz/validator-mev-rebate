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

// MockBuilderConfig mock builder 监听配置
type MockBuilderConfig struct {
	Name string `mapstructure:"name"`
	Addr string `mapstructure:"addr"`
}

// Config 全局配置
type Config struct {
	Server      ServerConfig      `mapstructure:"server"`
	Dispatcher  DispatcherConfig  `mapstructure:"dispatcher"`
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
}

func validate(cfg *Config) error {
	if cfg.Server.Port <= 0 || cfg.Server.Port > 65535 {
		return fmt.Errorf("server.port %d is out of range", cfg.Server.Port)
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
