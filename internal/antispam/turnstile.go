package antispam

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Turnstile struct {
	Secret, Hostname string
	Client           *http.Client
	Endpoint         string
}

func (t Turnstile) Verify(ctx context.Context, token string) (bool, error) {
	if token == "" || len(token) > 2048 {
		return false, nil
	}
	endpoint := t.Endpoint
	if endpoint == "" {
		endpoint = "https://challenges.cloudflare.com/turnstile/v0/siteverify"
	}
	form := url.Values{"secret": {t.Secret}, "response": {token}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return false, errors.New("captcha request unavailable")
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client := t.Client
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return false, errors.New("captcha service unavailable")
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 500 {
		return false, errors.New("captcha service unavailable")
	}
	if resp.StatusCode != http.StatusOK {
		return false, nil
	}
	var result struct {
		Success  bool     `json:"success"`
		Hostname string   `json:"hostname"`
		Errors   []string `json:"error-codes"`
	}
	if json.NewDecoder(io.LimitReader(resp.Body, 8192)).Decode(&result) != nil {
		return false, errors.New("captcha invalid response")
	}
	for _, code := range result.Errors {
		if code == "internal-error" {
			return false, errors.New("captcha service unavailable")
		}
	}
	return result.Success && result.Hostname == t.Hostname, nil
}
