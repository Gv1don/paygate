package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	ServerPort       int
	ScyllaAddr       string
	ScyllaKeyspace   string
	BankAPIURL       string
	BankSecret       string
	ThreeDSReturnURL string
	LogLevel         string
	CORSOrigin       string
	SwaggerHost      string
	FrontendURL      string
	RateLimitRPS     int
	RateLimitBurst   int
}

func Load() *Config {
	return &Config{
		ServerPort:     envInt("SERVER_PORT", 8081),
		ScyllaAddr:     envStr("SCYLLA_ADDR", "scylladb:9042"),
		ScyllaKeyspace: envStr("SCYLLA_KEYSPACE", "paygate"),
		BankAPIURL:       envStr("BANK_API_URL", "https://bankapi.example.com"),
		BankSecret:       envStr("BANK_SECRET", ""),
		ThreeDSReturnURL: envStr("THREE_DS_RETURN_URL", "http://localhost/api/v1/payments/3ds-return"),
		LogLevel:         envStr("LOG_LEVEL", "info"),
		CORSOrigin:     envStr("CORS_ORIGIN", "*"),
		SwaggerHost:    envStr("SWAGGER_HOST", "localhost:8081"),
		FrontendURL:    envStr("FRONTEND_URL", "http://localhost"),
		RateLimitRPS:   envInt("RATE_LIMIT_RPS", 100),
		RateLimitBurst: envInt("RATE_LIMIT_BURST", 200),
	}
}

func (c *Config) Addr() string {
	return fmt.Sprintf(":%d", c.ServerPort)
}

func envStr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
