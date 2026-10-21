package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"baso-lintang/internal/antispam"
	"baso-lintang/internal/config"
	"baso-lintang/internal/domain"
	apphttp "baso-lintang/internal/http"
	"baso-lintang/internal/jobs"
	"baso-lintang/internal/store"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(log)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, log, os.Args[1:]); err != nil {
		// Driver errors can include submitted values or connection credentials.
		log.Error("aplikasi berhenti; periksa konfigurasi dan koneksi database")
		os.Exit(1)
	}
}

func run(ctx context.Context, log *slog.Logger, args []string) error {
	if len(args) > 0 && args[0] == "migrate" {
		if len(args) != 2 || os.Getenv("DATABASE_URL") == "" {
			return errors.New("gunakan app migrate up|down dengan DATABASE_URL")
		}
		return store.Migrate(ctx, os.Getenv("DATABASE_URL"), args[1])
	}
	if len(args) >= 2 && args[0] == "admin" && args[1] == "create" {
		flags := flag.NewFlagSet("admin create", flag.ContinueOnError)
		email := flags.String("email", "", "Email admin")
		name := flags.String("name", "", "Nama admin")
		if err := flags.Parse(args[2:]); err != nil {
			return err
		}
		s, err := store.Open(ctx, os.Getenv("DATABASE_URL"), domain.Normalizer{})
		if err != nil {
			return err
		}
		defer s.Pool.Close()
		// Password is read from the environment, never a process argument.
		if err = s.CreateAdmin(ctx, *email, *name, os.Getenv("ADMIN_PASSWORD")); err != nil {
			return err
		}
		log.Info("akun admin dibuat")
		return nil
	}
	if len(args) > 0 && args[0] != "serve" {
		return errors.New("perintah tidak dikenal")
	}
	c, err := config.Load()
	if err != nil {
		return err
	}
	s, err := store.Open(ctx, c.DatabaseURL, domain.Normalizer{Replacements: c.Replacements})
	if err != nil {
		return err
	}
	defer s.Pool.Close()
	service := domain.SubmissionService{Repo: s, Captcha: antispam.Turnstile{Secret: c.CaptchaSecret, Hostname: c.CaptchaHostname},
		Hour: c.Hour, Day: c.Day, OutageHour: c.OutageHour, OutageDay: c.OutageDay}
	app, err := apphttp.New(s, c, service, log)
	if err != nil {
		return err
	}
	server := func(addr string, h http.Handler) *http.Server {
		return &http.Server{Addr: addr, Handler: h,
			ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 20 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 * 1024}
	}
	public, ops := server(c.Address, app.Handler()), server(c.OpsAddress, app.Operations())
	jobCtx, cancelJobs := context.WithCancel(ctx)
	var wg sync.WaitGroup
	wg.Go(func() { jobs.Run(jobCtx, s, log) })
	defer func() { cancelJobs(); wg.Wait() }()
	errs := make(chan error, 2)
	go func() { errs <- public.ListenAndServe() }()
	go func() { errs <- ops.ListenAndServe() }()
	log.Info("server dimulai")
	var serveErr error
	select {
	case <-ctx.Done():
	case serveErr = <-errs:
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), c.ShutdownTimeout)
	defer cancel()
	err1 := public.Shutdown(shutdownCtx)
	err2 := ops.Shutdown(shutdownCtx)
	if errors.Is(serveErr, http.ErrServerClosed) {
		serveErr = nil
	}
	return errors.Join(serveErr, err1, err2)
}
