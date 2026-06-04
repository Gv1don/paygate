package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config содержит все настройки приложения.
type Config struct {
	ServerPort  int
	ScyllaAddr  string
	ScyllaKeyspace string
	BankAPIURL  string
	BankSecret  string
	LogLevel    string
}

// Load читает конфигурацию из переменных окружения с значениями по умолчанию.
func Load() *Config {
	return &Config{
		ServerPort:     envInt("SERVER_PORT", 8081),
		ScyllaAddr:     envStr("SCYLLA_ADDR", "scylladb:9042"),
		ScyllaKeyspace: envStr("SCYLLA_KEYSPACE", "paygate"),
		BankAPIURL:     envStr("BANK_API_URL", "https://bankapi.example.com"),
		BankSecret:     envStr("BANK_SECRET", ""),
		LogLevel:       envStr("LOG_LEVEL", "info"),
	}
}

// Addr возвращает адрес для ListenAndServe.
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
