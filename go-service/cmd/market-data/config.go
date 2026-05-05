package main

import (
	"fmt"
	"log/slog"
	"os"
	"time"
)

type config struct {
	HTTPAddr        string
	ExchangeBaseURL string
	FetchTimeout    time.Duration
	CacheTTL        time.Duration
	LogLevel        slog.Level
}

func loadConfig() (*config, error) {
	c := &config{
		HTTPAddr:        getenvDefault("MD_HTTP_ADDR", ":8080"),
		ExchangeBaseURL: getenvDefault("EXCHANGE_BASE_URL", "https://api.binance.com"),
	}

	timeout, err := parseDurationEnv("MD_FETCH_TIMEOUT", "10s")
	if err != nil {
		return nil, err
	}
	if timeout <= 0 {
		return nil, fmt.Errorf("MD_FETCH_TIMEOUT must be positive, got %s", timeout)
	}
	c.FetchTimeout = timeout

	cacheTTL, err := parseDurationEnv("MD_CACHE_TTL", "30s")
	if err != nil {
		return nil, err
	}
	if cacheTTL < 0 {
		return nil, fmt.Errorf("MD_CACHE_TTL must be non-negative, got %s", cacheTTL)
	}
	c.CacheTTL = cacheTTL

	if err := c.LogLevel.UnmarshalText([]byte(getenvDefault("MD_LOG_LEVEL", "info"))); err != nil {
		return nil, fmt.Errorf("MD_LOG_LEVEL: %w", err)
	}

	return c, nil
}

func parseDurationEnv(key, def string) (time.Duration, error) {
	v := getenvDefault(key, def)
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	return d, nil
}

func getenvDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
