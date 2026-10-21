//go:build integration

package http

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"baso-lintang/internal/config"
	"baso-lintang/internal/domain"
	"baso-lintang/internal/store/dbgen"
	"baso-lintang/internal/testutil"
)

type acceptedCaptcha struct{}

func (acceptedCaptcha) Verify(context.Context, string) (bool, error) { return true, nil }

func TestHTTPSubmissionModerationAndSecurity(t *testing.T) {
	s := testutil.Database(t)
	ctx := context.Background()
	password := "password-uji-bukan-rahasia"
	if err := s.CreateAdmin(ctx, "admin@example.invalid", "Admin uji", password); err != nil {
		t.Fatal(err)
	}
	admin, err := s.Q.GetUserByEmail(ctx, "admin@example.invalid")
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{BaseURL: "https://example.invalid", SessionSecret: strings.Repeat("s", 32), HashSalt: strings.Repeat("h", 32)}
	app, err := New(s, cfg, domain.SubmissionService{Repo: s, Captcha: acceptedCaptcha{}, Hour: 100, Day: 200, OutageHour: 2, OutageDay: 5}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	handler := app.Handler()
	request := func(method, path string, v url.Values, cookies []*http.Cookie) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, "https://example.invalid"+path, strings.NewReader(v.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.Header.Set("Origin", cfg.BaseURL)
		for _, c := range cookies {
			r.AddCookie(c)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	csrfPattern := regexp.MustCompile(`name="csrf" value="([^"]+)"`)
	csrf := func(w *httptest.ResponseRecorder) string {
		t.Helper()
		m := csrfPattern.FindStringSubmatch(w.Body.String())
		if len(m) != 2 {
			t.Fatal("missing csrf form", w.Code)
		}
		return m[1]
	}
	login := func(email string) []*http.Cookie {
		t.Helper()
		get := request("GET", "/admin/login", nil, nil)
		post := request("POST", "/admin/login", url.Values{"email": {email}, "password": {password}, "csrf": {csrf(get)}}, get.Result().Cookies())
		if post.Code != 303 {
			t.Fatal("login", post.Code, post.Body.String())
		}
		var cookies []*http.Cookie
		for _, c := range post.Result().Cookies() {
			if c.Name == sessionName {
				if !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteLaxMode {
					t.Fatal("insecure cookie")
				}
				cookies = append(cookies, c)
			}
		}
		return cookies
	}
	// Artificial fixtures only, never claimed to be Lintang vocabulary.
	form := url.Values{"headword": {"kata-uji-http"}, "gloss": {`arti uji <script>alert(1)</script>`}, "name": {"Nama rahasia uji"}, "contact": {"kontak@example.invalid"}}
	if w := request("POST", "/kirim", form, nil); w.Code != 200 {
		t.Fatal("submit", w.Code, w.Body.String())
	}
	before := request("GET", "/cari?q=kataujihttp&arah=lintang", nil, nil)
	if strings.Contains(before.Body.String(), "kata-uji-http") {
		t.Fatal("pending publicly visible")
	}
	bad := url.Values{"headword": {"kata-uji-invalid"}, "gloss": {""}}
	if w := request("POST", "/kirim", bad, nil); w.Code != 422 || !strings.Contains(w.Body.String(), "kata-uji-invalid") {
		t.Fatal("form not preserved")
	}
	cookies := login("admin@example.invalid")
	q, err := s.Q.Queue(ctx, dbgen.QueueParams{Status: "pending", PageLimit: 20})
	if err != nil || len(q) != 1 {
		t.Fatal("queue", err)
	}
	path := "/admin/kiriman/" + strconv.FormatInt(q[0].ID, 10)
	if w := request("POST", path, form, cookies); w.Code != 403 {
		t.Fatal("CSRF missing accepted")
	}
	review := request("GET", path, nil, cookies)
	form.Set("csrf", csrf(review))
	form.Set("action", "approve")
	form.Set("origin", "serapan_en")
	if w := request("POST", path, form, cookies); w.Code != 303 {
		t.Fatal("review", w.Code, w.Body.String())
	}
	sub, err := s.Q.GetSubmission(ctx, q[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	detailPath := "/kata/kataujihttp-" + strconv.FormatInt(q[0].ID, 10)
	detail := request("GET", detailPath, nil, nil)
	body := detail.Body.String()
	if detail.Code != 200 || !strings.Contains(body, "&lt;script&gt;") || strings.Contains(body, "<script>alert") {
		t.Fatal("XSS escape failed", detail.Code, body)
	}
	if strings.Contains(body, "kontak@example.invalid") || strings.Contains(body, "Nama rahasia") {
		t.Fatal("private contributor data exposed")
	}
	entry, err := s.Q.PublicEntry(ctx, "kataujihttp-"+strconv.FormatInt(q[0].ID, 10))
	if err != nil || entry.Origin != "belum_diketahui" {
		t.Fatal("origin mass assignment", err)
	}
	for _, state := range []struct {
		visibility string
		sensitive  bool
	}{{"hidden", false}, {"published", true}} {
		if _, err = s.Pool.Exec(ctx, "UPDATE entries SET visibility=$2,is_sensitive=$3 WHERE id=$1", sub.ResultingEntryID.Int64, state.visibility, state.sensitive); err != nil {
			t.Fatal(err)
		}
		for _, publicPath := range []string{"/cari?q=kataujihttp&arah=lintang", "/cari?q=arti&arah=indonesia", "/daftar", "/daftar/k"} {
			w := request("GET", publicPath, nil, nil)
			if w.Code != 200 || strings.Contains(w.Body.String(), "kata-uji-http") {
				t.Fatal("hidden leak", publicPath)
			}
		}
		if w := request("GET", detailPath, nil, nil); w.Code != 404 {
			t.Fatal("hidden detail leak")
		}
	}
	for _, path := range []string{"/register", "/daftar-akun", "/api/v1/entries", "/healthz", "/metrics"} {
		if w := request("GET", path, nil, nil); w.Code != 404 {
			t.Fatal("unexpected public endpoint", path, w.Code)
		}
	}
	token, err := s.Invite(ctx, admin.ID, "verifier@example.invalid", "verifier")
	if err != nil {
		t.Fatal(err)
	}
	get := request("GET", "/admin/undangan?token="+token, nil, nil)
	accept := request("POST", "/admin/undangan", url.Values{"csrf": {csrf(get)}, "token": {token}, "name": {"Verifikator uji"}, "password": {password}}, get.Result().Cookies())
	if accept.Code != 303 {
		t.Fatal("invitation HTTP", accept.Code)
	}
	verifierCookies := login("verifier@example.invalid")
	if w := request("GET", "/admin/pengguna", nil, verifierCookies); w.Code != 403 {
		t.Fatal("verifier read users")
	}
	team := request("GET", "/admin/antrean", nil, verifierCookies)
	if w := request("POST", "/admin/pengguna", url.Values{"csrf": {csrf(team)}, "action": {"invite"}, "email": {"bad@example.invalid"}, "role": {"admin"}}, verifierCookies); w.Code != 403 {
		t.Fatal("verifier created admin")
	}
	for _, header := range []string{"Content-Security-Policy", "X-Content-Type-Options", "Referrer-Policy", "Strict-Transport-Security", "Permissions-Policy"} {
		if detail.Header().Get(header) == "" {
			t.Fatal("missing security header", header)
		}
	}
	if w := request("POST", "/admin/logout", url.Values{"csrf": {csrf(team)}}, verifierCookies); w.Code != 303 {
		t.Fatal("logout failed")
	}
	if w := request("GET", "/admin/antrean", nil, verifierCookies); w.Code != 303 {
		t.Fatal("logged out session retained")
	}
}
