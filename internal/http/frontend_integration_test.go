//go:build integration

package http

import (
	"context"
	"io"
	"log/slog"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"baso-lintang/internal/config"
	"baso-lintang/internal/domain"
	"baso-lintang/internal/store/dbgen"
	"baso-lintang/internal/testutil"
)

// TestFrontendBrowser uses a disposable database and a test-only CAPTCHA double.
// Neither fixtures nor the test CAPTCHA are available in the application binary.
func TestFrontendBrowser(t *testing.T) {
	if os.Getenv("UI_SMOKE") != "1" {
		t.Skip("set UI_SMOKE=1 after npm ci to run browser verification")
	}
	s := testutil.Database(t)
	ctx := context.Background()
	const password = "password-uji-bukan-rahasia"
	if err := s.CreateAdmin(ctx, "admin@example.invalid", "Admin uji", password); err != nil {
		t.Fatal(err)
	}
	admin, err := s.Q.GetUserByEmail(ctx, "admin@example.invalid")
	if err != nil {
		t.Fatal(err)
	}
	service := domain.SubmissionService{Repo: s, Captcha: acceptedCaptcha{}, Hour: 100, Day: 200, OutageHour: 2, OutageDay: 5}
	// Explicitly artificial vocabulary, used only inside this temporary database.
	for _, word := range []string{"kata-uji-1", "kata-uji-2", "kata-uji-3"} {
		in := domain.Submission{Headword: word, Gloss: "Arti buatan untuk pengujian", Explanation: "Placeholder pengujian, bukan kosakata Lintang.", POS: "Data uji"}
		if _, err := s.SaveSubmission(ctx, in); err != nil {
			t.Fatal(err)
		}
		pending, err := s.Q.Queue(ctx, dbgen.QueueParams{Status: "pending", PageLimit: 20})
		if err != nil || len(pending) != 1 {
			t.Fatal("fixture queue", err)
		}
		if err := s.Review(ctx, domain.Review{SubmissionID: pending[0].ID, ReviewerID: admin.ID, Action: "approve", Content: in}); err != nil {
			t.Fatal(err)
		}
	}
	cfg := config.Config{SessionSecret: strings.Repeat("s", 32), HashSalt: strings.Repeat("h", 32)}
	app, err := New(s, cfg, service, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewUnstartedServer(app.Handler())
	server.Config.ErrorLog = slog.NewLogLogger(slog.NewTextHandler(io.Discard, nil), slog.LevelError)
	server.StartTLS()
	defer server.Close()
	app.Config.BaseURL = server.URL
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	command := exec.CommandContext(ctx, "node", "scripts/ui-smoke.cjs")
	command.Dir = root
	command.Env = append(os.Environ(), "UI_BASE_URL="+server.URL)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("browser verification: %v\n%s", err, output)
	}
	t.Log(string(output))
}
