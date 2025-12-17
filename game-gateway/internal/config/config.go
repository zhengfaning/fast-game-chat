package config

import (
	"log"

	"github.com/spf13/viper"
)

type Config struct {
	Server struct {
		Host string `mapstructure:"host"`
		Port int    `mapstructure:"port"`
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
	viper.SetConfigName("gateway")
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
