package config

import (
	"flag"
	"log"

	"github.com/spf13/viper"
)

type Config struct {
	Server struct {
		Host string `mapstructure:"host"`
		Port int    `mapstructure:"port"`
		Env  string `mapstructure:"env"`
	} `mapstructure:"server"`

	Games []GameConfig `mapstructure:"games"`

	Redis struct {
		Addr     string `mapstructure:"addr"`
		Password string `mapstructure:"password"`
	} `mapstructure:"redis"`
}

type GameConfig struct {
	ID          string        `mapstructure:"id"`
	GameBackend BackendConfig `mapstructure:"game_backend"`
	ChatBackend BackendConfig `mapstructure:"chat_backend"`
}

type BackendConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	PoolSize int    `mapstructure:"pool_size"`
}

func Load() (*Config, error) {
	// 支持 -config 命令行参数
	var configPath string
	flag.StringVar(&configPath, "config", "", "配置文件路径")
	flag.Parse()

	if configPath != "" {
		// 如果指定了配置文件路径，直接使用
		viper.SetConfigFile(configPath)
	} else {
		// 否则使用默认搜索逻辑
		viper.SetConfigName("gateway")
		viper.SetConfigType("yaml")
		viper.AddConfigPath("configs")
		viper.AddConfigPath(".")
	}

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		log.Printf("Warning: Config file not found, using defaults or env vars: %v", err)
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
