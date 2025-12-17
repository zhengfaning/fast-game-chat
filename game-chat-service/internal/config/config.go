package config

import (
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
	viper.SetConfigName("chat")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("configs")
	viper.AddConfigPath(".")

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
