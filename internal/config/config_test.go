package config

import (
	"strings"
	"testing"
)

func TestConfigurationFailsClosed(t *testing.T) {
	for key, value := range map[string]string{
		"DATABASE_URL": "postgres://example.invalid/test", "BASE_URL": "https://example.invalid",
		"SESSION_SECRET": strings.Repeat("s", 32), "HASH_SALT": strings.Repeat("h", 32),
		"TURNSTILE_SITE_KEY": "test", "TURNSTILE_SECRET": "test",
		"RATE_LIMIT_HOUR": "10", "RATE_LIMIT_DAY": "30", "RATE_LIMIT_OUTAGE_HOUR": "2", "RATE_LIMIT_OUTAGE_DAY": "5",
		"NORMALIZATION_REPLACEMENTS": `["x","z"]`,
	} {
		t.Setenv(key, value)
	}
	if _, err := Load(); err != nil {
		t.Fatal(err)
	}
	t.Setenv("BASE_URL", "http://example.invalid")
	if _, err := Load(); err == nil {
		t.Fatal("insecure origin accepted")
	}
	t.Setenv("BASE_URL", "https://example.invalid")
	t.Setenv("RATE_LIMIT_OUTAGE_HOUR", "10")
	if _, err := Load(); err == nil {
		t.Fatal("outage limits not tighter")
	}
	t.Setenv("RATE_LIMIT_OUTAGE_HOUR", "2")
	t.Setenv("NORMALIZATION_REPLACEMENTS", `["x"]`)
	if _, err := Load(); err == nil {
		t.Fatal("odd replacement list accepted")
	}
	t.Setenv("NORMALIZATION_REPLACEMENTS", `["","z"]`)
	if _, err := Load(); err == nil {
		t.Fatal("empty replacement source accepted")
	}
}
