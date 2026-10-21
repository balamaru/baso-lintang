package antispam

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTurnstileValidation(t *testing.T) {
	for _, tc := range []struct {
		name, body         string
		status             int
		valid, unavailable bool
	}{
		{"valid", `{"success":true,"hostname":"example.invalid"}`, 200, true, false},
		{"wrong host", `{"success":true,"hostname":"other.invalid"}`, 200, false, false},
		{"replay", `{"success":false,"error-codes":["timeout-or-duplicate"]}`, 200, false, false},
		{"configuration", `{"success":false,"error-codes":["invalid-input-secret"]}`, 200, false, false},
		{"outage", `{}`, 503, false, true},
		{"internal", `{"success":false,"error-codes":["internal-error"]}`, 200, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if err := r.ParseForm(); err != nil {
					t.Error(err)
				}
				if r.Form.Has("remoteip") {
					t.Error("raw IP must not leave app")
				}
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()
			c := Turnstile{Secret: "test", Hostname: "example.invalid", Endpoint: server.URL, Client: server.Client()}
			valid, err := c.Verify(context.Background(), "test-token")
			if valid != tc.valid || (err != nil) != tc.unavailable {
				t.Fatalf("valid=%v err=%v", valid, err)
			}
		})
	}
}
