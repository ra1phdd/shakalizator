package config

import (
	"github.com/caarlos0/env"
	"github.com/joho/godotenv"
	"log"
)

var cfg Config

type Config struct {
	TelegramAPI string `env:"TELEGRAM_API,required"`
	LoggerLevel string `env:"LOGGER_LEVEL" envDefault:"warn"`
}

func NewConfig(files ...string) (*Config, error) {
	err := godotenv.Load(files...)
	if err != nil {
		log.Fatal("Файл .env не найден", err.Error())
	}

	err = env.Parse(&cfg)
	if err != nil {
		return nil, err
	}

	return &cfg, nil
}

func Get() *Config {
	return &cfg
}
