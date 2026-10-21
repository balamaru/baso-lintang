package http

import (
	"bytes"
	"context"
	"crypto/subtle"
	"errors"
	"html/template"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"baso-lintang/internal/auth"
	"baso-lintang/internal/config"
	"baso-lintang/internal/domain"
	"baso-lintang/internal/store"
	"baso-lintang/internal/store/dbgen"
	"baso-lintang/web"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

type contextKey int

const userKey contextKey = 0
const csrfKey contextKey = 1
const sessionName = "__Host-lintang-session"
const csrfName = "__Host-lintang-csrf"

type App struct {
	Store       *store.Store
	Config      config.Config
	Submissions domain.SubmissionService
	Log         *slog.Logger
	templates   *template.Template
	requests    atomic.Uint64
	failures    atomic.Uint64
}

type page struct {
	Kind, Title, Error, Message, CSRF, SiteKey, Next string
	Data                                             any
	Form                                             url.Values
	User                                             dbgen.User
}

func New(s *store.Store, c config.Config, service domain.SubmissionService, log *slog.Logger) (*App, error) {
	t, err := template.ParseFS(web.Templates, "templates/*.html")
	if err != nil {
		return nil, err
	}
	return &App{Store: s, Config: c, Submissions: service, Log: log, templates: t}, nil
}

func (a *App) Handler() http.Handler {
	r := chi.NewRouter()
	r.Use(a.security)
	r.Get("/", a.search)
	r.Get("/cari", a.search)
	r.Get("/daftar", a.list)
	r.Get("/daftar/{huruf}", a.list)
	r.Get("/kata/{slug}", a.detail)
	r.Get("/kirim", a.submission)
	r.Post("/kirim", a.submission)
	r.Get("/admin/login", a.login)
	r.Post("/admin/login", a.login)
	r.Get("/admin/undangan", a.invitation)
	r.Post("/admin/undangan", a.invitation)
	r.Group(func(r chi.Router) {
		r.Use(a.requireTeam)
		r.Post("/admin/logout", a.logout)
		r.Get("/admin/antrean", a.queue)
		r.Get("/admin/kiriman/{id}", a.review)
		r.Post("/admin/kiriman/{id}", a.review)
		r.Group(func(r chi.Router) {
			r.Use(a.requireAdmin)
			r.Get("/admin/pengguna", a.users)
			r.Post("/admin/pengguna", a.users)
		})
	})
	return r
}

func (a *App) security(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		a.requests.Add(1)
		w.Header().Set("Content-Security-Policy", "default-src 'none'; script-src https://challenges.cloudflare.com; frame-src https://challenges.cloudflare.com; style-src 'self'; img-src 'self'; connect-src 'self' https://challenges.cloudflare.com; form-action 'self'; base-uri 'none'; frame-ancestors 'none'")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Strict-Transport-Security", "max-age=31536000")
		w.Header().Set("Permissions-Policy", "microphone=(), camera=(), geolocation=()")
		w.Header().Set("Cache-Control", "no-store")
		defer func() {
			if recover() != nil {
				a.failures.Add(1)
				a.Log.Error("request panic recovered")
				http.Error(w, "Terjadi kesalahan internal.", 500)
			}
		}()
		r.Body = http.MaxBytesReader(w, r.Body, 32*1024)
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		r = r.WithContext(ctx)
		if len(r.URL.RawQuery) > 4096 {
			http.Error(w, "Permintaan terlalu panjang.", 414)
			return
		}
		if r.Method == http.MethodPost {
			if r.Header.Get("Origin") != "" && r.Header.Get("Origin") != a.Config.BaseURL {
				http.Error(w, "Asal permintaan tidak valid.", 403)
				return
			}
			if r.Header.Get("Sec-Fetch-Site") == "cross-site" {
				http.Error(w, "Asal permintaan tidak valid.", 403)
				return
			}
			if err := r.ParseForm(); err != nil {
				http.Error(w, "Form terlalu besar atau tidak valid.", 400)
				return
			}
		}
		var user dbgen.User
		session := cookie(r, sessionName)
		if session != "" {
			u, err := a.Store.Q.SessionUser(ctx, auth.TokenHash(session))
			if err != nil && !errors.Is(err, pgx.ErrNoRows) {
				a.fail(w, r, err)
				return
			}
			if err == nil {
				user = u
				if err = a.Store.Q.TouchSession(ctx, auth.TokenHash(session)); err != nil {
					a.fail(w, r, err)
					return
				}
			}
		}
		seed := session
		if user.ID == 0 {
			seed = cookie(r, csrfName)
			if len(seed) != 43 {
				var err error
				seed, err = auth.Token()
				if err != nil {
					a.fail(w, r, err)
					return
				}
				setCookie(w, csrfName, seed, 3600)
			}
		}
		csrf := auth.Hash(a.Config.SessionSecret, "csrf", seed)
		if r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/admin/") &&
			subtle.ConstantTimeCompare([]byte(r.PostForm.Get("csrf")), []byte(csrf)) != 1 {
			http.Error(w, "Token keamanan tidak valid. Muat ulang form.", 403)
			return
		}
		ctx = context.WithValue(ctx, userKey, user)
		ctx = context.WithValue(ctx, csrfKey, csrf)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func currentUser(r *http.Request) dbgen.User {
	u, _ := r.Context().Value(userKey).(dbgen.User)
	return u
}
func cookie(r *http.Request, name string) string {
	c, err := r.Cookie(name)
	if err != nil {
		return ""
	}
	return c.Value
}
func setCookie(w http.ResponseWriter, name, value string, age int) {
	http.SetCookie(w, &http.Cookie{Name: name, Value: value, Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: age})
}
func (a *App) requireTeam(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if currentUser(r).ID == 0 {
			http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r)
	})
}
func (a *App) requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if currentUser(r).Role != "admin" {
			http.Error(w, "Akses hanya untuk admin.", 403)
			return
		}
		next.ServeHTTP(w, r)
	})
}
func (a *App) render(w http.ResponseWriter, r *http.Request, status int, p page) {
	p.User = currentUser(r)
	p.CSRF, _ = r.Context().Value(csrfKey).(string)
	p.SiteKey = a.Config.SiteKey
	if p.Form == nil {
		p.Form = url.Values{}
	}
	var b bytes.Buffer
	if err := a.templates.ExecuteTemplate(&b, "page", p); err != nil {
		a.fail(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if _, err := w.Write(b.Bytes()); err != nil {
		a.Log.Warn("response write failed")
	}
}
func (a *App) fail(w http.ResponseWriter, r *http.Request, err error) {
	status := 500
	message := "Terjadi kesalahan. Silakan coba lagi."
	switch {
	case errors.Is(err, pgx.ErrNoRows), errors.Is(err, domain.ErrNotFound):
		status = 404
		message = "Data tidak ditemukan."
	case errors.Is(err, domain.ErrInvalid):
		status = 422
		message = domain.ErrInvalid.Error()
	case errors.Is(err, domain.ErrForbidden):
		status = 403
		message = domain.ErrForbidden.Error()
	case errors.Is(err, domain.ErrConflict):
		status = 409
		message = domain.ErrConflict.Error()
	}
	if status == 500 {
		a.failures.Add(1)
		a.Log.Error("request failed", "method", r.Method)
	}
	http.Error(w, message, status)
}
func pagination(r *http.Request) (int32, string) {
	n, _ := strconv.Atoi(r.URL.Query().Get("halaman"))
	if n < 1 {
		n = 1
	}
	if n > 10000 {
		n = 10000
	}
	q := r.URL.Query()
	q.Set("halaman", strconv.Itoa(n+1))
	return int32((n - 1) * 20), r.URL.Path + "?" + q.Encode()
}
func (a *App) search(w http.ResponseWriter, r *http.Request) {
	query := a.Store.Normalizer.Normalize(r.URL.Query().Get("q"))
	direction := r.URL.Query().Get("arah")
	if direction == "" {
		direction = "semua"
	}
	if direction != "semua" && direction != "lintang" && direction != "indonesia" {
		a.fail(w, r, domain.ErrInvalid)
		return
	}
	offset, next := pagination(r)
	rows := []dbgen.SearchEntriesRow{}
	if query != "" {
		var err error
		rows, err = a.Store.Q.SearchEntries(r.Context(), dbgen.SearchEntriesParams{Query: query, Direction: direction, PageLimit: 20, PageOffset: offset})
		if err != nil {
			a.fail(w, r, err)
			return
		}
	}
	if len(rows) < 20 {
		next = ""
	}
	a.render(w, r, 200, page{Kind: "search", Title: "Kamus Bahasa Lintang", Data: rows, Form: r.URL.Query(), Next: next})
}
func (a *App) list(w http.ResponseWriter, r *http.Request) {
	offset, next := pagination(r)
	rows, err := a.Store.Q.ListEntries(r.Context(), dbgen.ListEntriesParams{Prefix: a.Store.Normalizer.Normalize(chi.URLParam(r, "huruf")), PageLimit: 20, PageOffset: offset})
	if err != nil {
		a.fail(w, r, err)
		return
	}
	if len(rows) < 20 {
		next = ""
	}
	a.render(w, r, 200, page{Kind: "list", Title: "Daftar kata", Data: rows, Next: next})
}
func (a *App) detail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	e, err := a.Store.Q.PublicEntry(ctx, chi.URLParam(r, "slug"))
	if err != nil {
		a.fail(w, r, err)
		return
	}
	spellings, err := a.Store.Q.PublicSpellings(ctx, e.ID)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	senses, err := a.Store.Q.PublicSenses(ctx, e.ID)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	examples, err := a.Store.Q.PublicExamples(ctx, e.ID)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	credits, err := a.Store.Q.PublicCredits(ctx, e.ID)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	a.render(w, r, 200, page{Kind: "detail", Title: e.Headword, Data: struct {
		Entry     dbgen.Entry
		Spellings []dbgen.Spelling
		Senses    []dbgen.PublicSensesRow
		Examples  []dbgen.Example
		Credits   []string
	}{e, spellings, senses, examples, credits}})
}
func formSubmission(v url.Values) domain.Submission {
	return domain.Submission{Headword: strings.TrimSpace(v.Get("headword")), Gloss: strings.TrimSpace(v.Get("gloss")),
		Explanation: v.Get("explanation"), POS: v.Get("pos"), ExampleLintang: v.Get("example_lintang"), ExampleID: v.Get("example_id"),
		Name: v.Get("name"), Contact: v.Get("contact"), Consent: v.Get("consent") == "on"}
}
func (a *App) ipHash(r *http.Request) string {
	// Forwarded headers are deliberately ignored until a trusted ingress is configured.
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	return auth.Hash(a.Config.HashSalt, "ip", host)
}
func (a *App) submission(w http.ResponseWriter, r *http.Request) {
	p := page{Kind: "submission", Title: "Kirim kata", Form: r.PostForm}
	if r.Method == http.MethodPost {
		in := formSubmission(r.PostForm)
		in.IPHash = a.ipHash(r)
		in.FingerprintHash = auth.Hash(a.Config.HashSalt, "device", r.UserAgent()+"\x00"+r.Header.Get("Accept-Language"))
		in.UserID = currentUser(r).ID
		if in.UserID != 0 {
			in.SessionHash = auth.TokenHash(cookie(r, sessionName))
		}
		duplicate, err := a.Submissions.Submit(r.Context(), in, r.PostForm.Get("cf-turnstile-response"), r.PostForm.Get("website"))
		if err != nil {
			status := 422
			if errors.Is(err, domain.ErrLimited) {
				status = 429
			}
			if !errors.Is(err, domain.ErrLimited) && !errors.Is(err, domain.ErrInvalid) && !errors.Is(err, domain.ErrCaptcha) {
				a.fail(w, r, err)
				return
			}
			p.Error = err.Error()
			a.render(w, r, status, p)
			return
		}
		p.Kind = "message"
		p.Message = "Terima kasih. Kiriman Anda akan ditinjau sebelum ditampilkan."
		if duplicate {
			p.Message = "Kiriman sudah diterima dan sedang menunggu tinjauan."
		}
		a.render(w, r, 200, p)
		return
	}
	a.render(w, r, 200, p)
}
func (a *App) login(w http.ResponseWriter, r *http.Request) {
	p := page{Kind: "login", Title: "Login tim"}
	if r.Method == http.MethodPost {
		ok, err := a.Store.Allow(r.Context(), "login:"+a.ipHash(r), 30, 100)
		if err != nil {
			a.fail(w, r, err)
			return
		}
		if !ok {
			p.Error = domain.ErrLogin.Error()
			a.render(w, r, 429, p)
			return
		}
		email := strings.ToLower(strings.TrimSpace(r.PostForm.Get("email")))
		key := auth.Hash(a.Config.HashSalt, "login", email)
		token, err := a.Store.Login(r.Context(), email, r.PostForm.Get("password"), key, auth.TokenHash(cookie(r, sessionName)))
		if err != nil {
			if !errors.Is(err, domain.ErrLogin) {
				a.fail(w, r, err)
				return
			}
			p.Error = domain.ErrLogin.Error()
			a.render(w, r, 401, p)
			return
		}
		setCookie(w, sessionName, token, 0)
		setCookie(w, csrfName, "", -1)
		http.Redirect(w, r, "/admin/antrean", 303)
		return
	}
	a.render(w, r, 200, p)
}
func (a *App) logout(w http.ResponseWriter, r *http.Request) {
	if err := a.Store.Q.DeleteSession(r.Context(), auth.TokenHash(cookie(r, sessionName))); err != nil {
		a.fail(w, r, err)
		return
	}
	setCookie(w, sessionName, "", -1)
	http.Redirect(w, r, "/admin/login", 303)
}
func (a *App) invitation(w http.ResponseWriter, r *http.Request) {
	p := page{Kind: "invitation", Title: "Terima undangan", Form: r.URL.Query()}
	if r.Method == http.MethodPost {
		ok, err := a.Store.Allow(r.Context(), "invitation:"+a.ipHash(r), 10, 30)
		if err != nil {
			a.fail(w, r, err)
			return
		}
		if !ok {
			http.Error(w, "Coba lagi nanti.", 429)
			return
		}
		err = a.Store.AcceptInvitation(r.Context(), r.PostForm.Get("token"), r.PostForm.Get("name"), r.PostForm.Get("password"))
		if err != nil {
			p.Error = "Undangan atau isian tidak valid. Password minimal 12 karakter."
			p.Form = r.PostForm
			a.render(w, r, 422, p)
			return
		}
		http.Redirect(w, r, "/admin/login", 303)
		return
	}
	a.render(w, r, 200, p)
}
func (a *App) queue(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	if status == "" {
		status = "pending"
	}
	if status != "pending" && status != "approved" && status != "rejected" && status != "needs_fix" {
		a.fail(w, r, domain.ErrInvalid)
		return
	}
	offset, next := pagination(r)
	rows, err := a.Store.Q.Queue(r.Context(), dbgen.QueueParams{Status: status, Query: r.URL.Query().Get("q"), Newest: r.URL.Query().Get("urutan") == "baru", PageLimit: 20, PageOffset: offset})
	if err != nil {
		a.fail(w, r, err)
		return
	}
	if len(rows) < 20 {
		next = ""
	}
	a.render(w, r, 200, page{Kind: "queue", Title: "Antrean kiriman", Data: rows, Next: next, Form: r.URL.Query()})
}
func (a *App) review(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id < 1 {
		a.fail(w, r, domain.ErrNotFound)
		return
	}
	sub, err := a.Store.Q.GetSubmission(r.Context(), id)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	f := url.Values{"headword": {sub.Headword}, "gloss": {sub.GlossID}, "explanation": {sub.Explanation}, "pos": {sub.Pos}, "example_lintang": {sub.ExampleLintang}, "example_id": {sub.ExampleID}}
	p := page{Kind: "review", Title: "Tinjau kiriman", Form: f, Data: sub}
	if r.Method == http.MethodPost {
		target, _ := strconv.ParseInt(r.PostForm.Get("target"), 10, 64)
		var variants []string
		for _, v := range strings.Split(r.PostForm.Get("variants"), ",") {
			if v = strings.TrimSpace(v); v != "" {
				variants = append(variants, v)
			}
		}
		err = a.Store.Review(r.Context(), domain.Review{SubmissionID: id, ReviewerID: currentUser(r).ID,
			SessionHash: auth.TokenHash(cookie(r, sessionName)), Action: r.PostForm.Get("action"), Note: r.PostForm.Get("note"),
			TargetEntryID: target, Content: formSubmission(r.PostForm), Variants: variants})
		if err != nil {
			if errors.Is(err, domain.ErrInvalid) {
				p.Form = r.PostForm
				p.Error = err.Error()
				a.render(w, r, 422, p)
				return
			}
			a.fail(w, r, err)
			return
		}
		http.Redirect(w, r, "/admin/antrean", 303)
		return
	}
	a.render(w, r, 200, p)
}
func (a *App) users(w http.ResponseWriter, r *http.Request) {
	message := ""
	if r.Method == http.MethodPost {
		if r.PostForm.Get("action") == "invite" {
			token, err := a.Store.Invite(r.Context(), currentUser(r).ID, strings.TrimSpace(r.PostForm.Get("email")), r.PostForm.Get("role"))
			if err != nil {
				a.fail(w, r, err)
				return
			}
			message = "Tautan undangan (berlaku 72 jam): " + a.Config.BaseURL + "/admin/undangan?token=" + url.QueryEscape(token)
		} else if r.PostForm.Get("action") == "update" {
			id, err := strconv.ParseInt(r.PostForm.Get("id"), 10, 64)
			if err != nil {
				a.fail(w, r, domain.ErrInvalid)
				return
			}
			if err = a.Store.UpdateUser(r.Context(), currentUser(r).ID, id, r.PostForm.Get("role"), r.PostForm.Get("active") == "on"); err != nil {
				a.fail(w, r, err)
				return
			}
			message = "Akun diperbarui dan sesi lama dicabut."
		} else {
			a.fail(w, r, domain.ErrInvalid)
			return
		}
	}
	offset, next := pagination(r)
	rows, err := a.Store.Q.ListUsers(r.Context(), offset)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	if len(rows) < 20 {
		next = ""
	}
	a.render(w, r, 200, page{Kind: "users", Title: "Pengguna tim", Data: rows, Message: message, Next: next})
}

// Operations is bound to a separate internal-only listener.
func (a *App) Operations() http.Handler {
	r := chi.NewRouter()
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) })
	r.Get("/readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if a.Store.Pool.Ping(ctx) != nil {
			http.Error(w, "Belum siap.", 503)
			return
		}
		w.WriteHeader(204)
	})
	r.Get("/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		_, err := w.Write([]byte("lintang_http_requests_total " + strconv.FormatUint(a.requests.Load(), 10) + "\nlintang_http_errors_total " + strconv.FormatUint(a.failures.Load(), 10) + "\n"))
		if err != nil {
			a.Log.Warn("metrics write failed")
		}
	})
	return r
}
