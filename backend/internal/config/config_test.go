package config

import (
	"os"
	"testing"
)

func TestLoad_Defaults(t *testing.T) {
	os.Clearenv()
	cfg := Load()

	if cfg.ServerPort != 8081 {
		t.Errorf("expected default port 8081, got %d", cfg.ServerPort)
	}
	if cfg.ScyllaAddr != "scylladb:9042" {
		t.Errorf("expected default scylla addr, got %s", cfg.ScyllaAddr)
	}
	if cfg.ScyllaKeyspace != "paygate" {
		t.Errorf("expected default keyspace paygate, got %s", cfg.ScyllaKeyspace)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("expected default log level info, got %s", cfg.LogLevel)
	}
	if cfg.CORSOrigin != "*" {
		t.Errorf("expected default cors origin *, got %s", cfg.CORSOrigin)
	}
	if cfg.SwaggerHost != "localhost:8081" {
		t.Errorf("expected default swagger host localhost:8081, got %s", cfg.SwaggerHost)
	}
	if cfg.RateLimitRPS != 100 {
		t.Errorf("expected default rate limit 100, got %d", cfg.RateLimitRPS)
	}
	if cfg.RateLimitBurst != 200 {
		t.Errorf("expected default rate limit burst 200, got %d", cfg.RateLimitBurst)
	}
	if cfg.FrontendURL != "http://localhost" {
		t.Errorf("expected default frontend url http://localhost, got %s", cfg.FrontendURL)
	}
}

func TestLoad_FromEnv(t *testing.T) {
	os.Clearenv()
	os.Setenv("SERVER_PORT", "9090")
	os.Setenv("SCYLLA_ADDR", "localhost:9042")
	os.Setenv("SCYLLA_KEYSPACE", "test_paygate")
	os.Setenv("BANK_API_URL", "https://sandbox.bank.com")
	os.Setenv("BANK_SECRET", "super_secret_key")
	os.Setenv("LOG_LEVEL", "debug")
	os.Setenv("CORS_ORIGIN", "https://example.com")
	os.Setenv("SWAGGER_HOST", "paygate.example.com")
	os.Setenv("RATE_LIMIT_RPS", "50")
	os.Setenv("RATE_LIMIT_BURST", "100")
	os.Setenv("FRONTEND_URL", "https://paygate.example.com")

	cfg := Load()

	if cfg.ServerPort != 9090 {
		t.Errorf("expected port 9090, got %d", cfg.ServerPort)
	}
	if cfg.ScyllaAddr != "localhost:9042" {
		t.Errorf("expected localhost:9042, got %s", cfg.ScyllaAddr)
	}
	if cfg.ScyllaKeyspace != "test_paygate" {
		t.Errorf("expected test_paygate, got %s", cfg.ScyllaKeyspace)
	}
	if cfg.BankAPIURL != "https://sandbox.bank.com" {
		t.Errorf("expected sandbox bank url, got %s", cfg.BankAPIURL)
	}
	if cfg.BankSecret != "super_secret_key" {
		t.Errorf("expected super_secret_key, got %s", cfg.BankSecret)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("expected debug, got %s", cfg.LogLevel)
	}
	if cfg.CORSOrigin != "https://example.com" {
		t.Errorf("expected https://example.com, got %s", cfg.CORSOrigin)
	}
	if cfg.SwaggerHost != "paygate.example.com" {
		t.Errorf("expected paygate.example.com, got %s", cfg.SwaggerHost)
	}
	if cfg.RateLimitRPS != 50 {
		t.Errorf("expected rate limit 50, got %d", cfg.RateLimitRPS)
	}
	if cfg.RateLimitBurst != 100 {
		t.Errorf("expected rate limit burst 100, got %d", cfg.RateLimitBurst)
	}
	if cfg.FrontendURL != "https://paygate.example.com" {
		t.Errorf("expected frontend url https://paygate.example.com, got %s", cfg.FrontendURL)
	}
}

func TestAddr(t *testing.T) {
	cfg := &Config{ServerPort: 8443}
	expected := ":8443"
	if got := cfg.Addr(); got != expected {
		t.Errorf("Addr() = %s, want %s", got, expected)
	}
}
