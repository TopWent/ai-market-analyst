package main

import (
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestLoadConfig(t *testing.T) {
	cases := []struct {
		name    string
		env     map[string]string
		wantErr string
		check   func(t *testing.T, c *config)
	}{
		{
			name: "all defaults",
			env:  map[string]string{},
			check: func(t *testing.T, c *config) {
				if c.HTTPAddr != ":8080" {
					t.Errorf("HTTPAddr = %q", c.HTTPAddr)
				}
				if c.ExchangeBaseURL != "https://api.binance.com" {
					t.Errorf("ExchangeBaseURL = %q", c.ExchangeBaseURL)
				}
				if c.FetchTimeout != 10*time.Second {
					t.Errorf("FetchTimeout = %s", c.FetchTimeout)
				}
				if c.CacheTTL != 30*time.Second {
					t.Errorf("CacheTTL = %s, want 30s", c.CacheTTL)
				}
				if c.LogLevel != slog.LevelInfo {
					t.Errorf("LogLevel = %s", c.LogLevel)
				}
			},
		},
		{
			name: "cache disabled by zero ttl",
			env:  map[string]string{"MD_CACHE_TTL": "0s"},
			check: func(t *testing.T, c *config) {
				if c.CacheTTL != 0 {
					t.Errorf("CacheTTL = %s, want 0", c.CacheTTL)
				}
			},
		},
		{
			name:    "negative cache ttl rejected",
			env:     map[string]string{"MD_CACHE_TTL": "-1s"},
			wantErr: "must be non-negative",
		},
		{
			name: "addr override",
			env:  map[string]string{"MD_HTTP_ADDR": "127.0.0.1:9000"},
			check: func(t *testing.T, c *config) {
				if c.HTTPAddr != "127.0.0.1:9000" {
					t.Errorf("HTTPAddr = %q", c.HTTPAddr)
				}
			},
		},
		{
			name:    "bad timeout",
			env:     map[string]string{"MD_FETCH_TIMEOUT": "not-a-duration"},
			wantErr: "MD_FETCH_TIMEOUT",
		},
		{
			name:    "negative timeout",
			env:     map[string]string{"MD_FETCH_TIMEOUT": "-1s"},
			wantErr: "must be positive",
		},
		{
			name:    "bad log level",
			env:     map[string]string{"MD_LOG_LEVEL": "verbose"},
			wantErr: "MD_LOG_LEVEL",
		},
		{
			name: "debug log level",
			env:  map[string]string{"MD_LOG_LEVEL": "debug"},
			check: func(t *testing.T, c *config) {
				if c.LogLevel != slog.LevelDebug {
					t.Errorf("LogLevel = %s", c.LogLevel)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			clearEnv(t)
			for k, v := range tc.env {
				t.Setenv(k, v)
			}

			cfg, err := loadConfig()

			switch {
			case tc.wantErr != "" && err == nil:
				t.Fatalf("expected error containing %q, got nil", tc.wantErr)
			case tc.wantErr != "" && !strings.Contains(err.Error(), tc.wantErr):
				t.Fatalf("expected error containing %q, got %q", tc.wantErr, err.Error())
			case tc.wantErr == "" && err != nil:
				t.Fatalf("unexpected error: %v", err)
			}

			if tc.check != nil {
				if cfg == nil {
					t.Fatal("config is nil but no error")
				}
				tc.check(t, cfg)
			}
		})
	}
}

func clearEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"MD_HTTP_ADDR",
		"EXCHANGE_BASE_URL",
		"MD_FETCH_TIMEOUT",
		"MD_CACHE_TTL",
		"MD_LOG_LEVEL",
	} {
		t.Setenv(k, "")
	}
}
