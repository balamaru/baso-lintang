package config

import (
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"strconv"
	"time"
)

type Config struct {
	DatabaseURL, Address, OpsAddress, BaseURL, SessionSecret, HashSalt, SiteKey, CaptchaSecret, CaptchaHostname string
	Hour, Day, OutageHour, OutageDay                                                                            int
	Replacements                                                                                                []string
	ShutdownTimeout                                                                                             time.Duration
}

func Load() (Config, error) {
	c := Config{DatabaseURL: os.Getenv("DATABASE_URL"), Address: env("HTTP_ADDR", ":8080"), OpsAddress: env("OPS_ADDR", "127.0.0.1:9090"),
		BaseURL: os.Getenv("BASE_URL"), SessionSecret: os.Getenv("SESSION_SECRET"), HashSalt: os.Getenv("HASH_SALT"),
		SiteKey: os.Getenv("TURNSTILE_SITE_KEY"), CaptchaSecret: os.Getenv("TURNSTILE_SECRET"), ShutdownTimeout: 10 * time.Second}
	if c.DatabaseURL == "" || len(c.SessionSecret) < 32 || len(c.HashSalt) < 32 || c.SiteKey == "" || c.CaptchaSecret == "" {
		return c, errors.New("DATABASE_URL, rahasia minimal 32 karakter, dan konfigurasi Turnstile wajib diisi")
	}
	u, err := url.Parse(c.BaseURL)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return c, errors.New("BASE_URL wajib berupa origin HTTPS")
	}
	c.BaseURL = u.Scheme + "://" + u.Host
	c.CaptchaHostname = u.Hostname()
	values := []struct {
		name     string
		fallback int
		target   *int
	}{
		{"RATE_LIMIT_HOUR", 10, &c.Hour}, {"RATE_LIMIT_DAY", 30, &c.Day},
		{"RATE_LIMIT_OUTAGE_HOUR", 2, &c.OutageHour}, {"RATE_LIMIT_OUTAGE_DAY", 5, &c.OutageDay},
	}
	for _, v := range values {
		n, err := strconv.Atoi(env(v.name, strconv.Itoa(v.fallback)))
		if err != nil || n < 1 || n > 100000 {
			return c, errors.New("konfigurasi rate limit tidak valid")
		}
		*v.target = n
	}
	if c.OutageHour >= c.Hour || c.OutageDay >= c.Day {
		return c, errors.New("batas saat CAPTCHA tidak tersedia wajib lebih ketat")
	}
	if raw := os.Getenv("NORMALIZATION_REPLACEMENTS"); raw != "" {
		if err := json.Unmarshal([]byte(raw), &c.Replacements); err != nil || len(c.Replacements)%2 != 0 {
			return c, errors.New("NORMALIZATION_REPLACEMENTS wajib array pasangan teks")
		}
		for i := 0; i < len(c.Replacements); i += 2 {
			if c.Replacements[i] == "" {
				return c, errors.New("aturan normalisasi tidak boleh kosong")
			}
		}
	}
	return c, nil
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
