package config

import (
	"sync"

	"github.com/caarlos0/env"
)

type (
	Config struct {
		Port         string `env:"PORT" envDefault:"8080"`
		AllowOrigins string `env:"ALLOW_ORIGINS"`

		LogLevel string `env:"LOG_LEVEL" envDefault:"info"`
	}
)

var (
	once sync.Once

	Conf Config
)

func load() {
	if err := env.Parse(&Conf); err != nil {
		panic(err)
	}
}

func init() {
	once.Do(load)
}
