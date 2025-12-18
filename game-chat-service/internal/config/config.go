package config

import (
	"flag"
	"log"

	"github.com/spf13/viper"
)

type Config struct {
	Server struct {
		Port     int `mapstructure:"port"`
		GrpcPort int `mapstructure:"grpc_port"`
	} `mapstructure:"server"`

	Database struct {
		DSN string `mapstructure:"dsn"`
	} `mapstructure:"database"`

	Redis struct {
		Addr     string `mapstructure:"addr"`
		Password string `mapstructure:"password"`
	} `mapstructure:"redis"`
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
		viper.SetConfigName("chat")
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
