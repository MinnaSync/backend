package config

import "os"

var (
	WSPort         string
	WSAllowOrigins string
)

func Load() {
	var ok bool

	WSPort, ok = os.LookupEnv("WS_PORT")
	if !ok || WSPort == "" {
		WSPort = "8080"
	}

	WSAllowOrigins, ok = os.LookupEnv("WS_ALLOW_ORIGINS")
	if !ok || WSAllowOrigins == "" {
		panic("WS_ALLOW_ORIGINS is not set.")
	}
}
