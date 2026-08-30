package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/smtp"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ggem/coglab-manager-go/internal/audit"
	"github.com/ggem/coglab-manager-go/internal/auth"
	"github.com/ggem/coglab-manager-go/internal/db"
	"github.com/ggem/coglab-manager-go/internal/httpapi"
	"github.com/ggem/coglab-manager-go/internal/mail"
	"github.com/ggem/coglab-manager-go/internal/reminders"
)

func main() {
	if err := run(); err != nil {
		slog.Error("fatal", "error", err)
		os.Exit(1)
	}
}

func run() error {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return fmt.Errorf("DATABASE_URL environment variable is required")
	}

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		return fmt.Errorf("create connection pool: %w", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}

	queries := db.New(pool)
	server := httpapi.NewServer(
		auth.NewPasswordAuthenticator(queries),
		auth.NewSessionManager(queries, secureCookies()),
		audit.NewRecorder(queries),
		queries,
		logger,
	)

	srv := &http.Server{
		Addr:              addr(),
		Handler:           server.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	mailer, err := newMailer()
	if err != nil {
		return err
	}
	leadTime, err := familyReminderLeadTime()
	if err != nil {
		return err
	}
	hour, err := digestHour()
	if err != nil {
		return err
	}
	scheduler := reminders.NewScheduler(queries, mailer, logger, leadTime, hour)
	scheduler.Run(ctx)

	errCh := make(chan error, 1)
	go func() {
		logger.Info("listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		logger.Info("shutdown signal received")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	shutdownErr := srv.Shutdown(shutdownCtx)
	scheduler.Wait()
	return shutdownErr
}

// newMailer builds the SMTP sender the scheduled jobs send through, from
// SMTP_ADDR ("host:port") and SMTP_FROM, both required the same way
// DATABASE_URL is -- this is core functionality being wired at startup,
// not an optional add-on that should silently degrade. SMTP_USERNAME/
// SMTP_PASSWORD are optional: a relay that needs no authentication (e.g.
// a local relay on the same host, matching legacy's own setup) can leave
// both unset.
func newMailer() (mail.Sender, error) {
	smtpAddr := os.Getenv("SMTP_ADDR")
	if smtpAddr == "" {
		return nil, fmt.Errorf("SMTP_ADDR environment variable is required")
	}
	smtpFrom := os.Getenv("SMTP_FROM")
	if smtpFrom == "" {
		return nil, fmt.Errorf("SMTP_FROM environment variable is required")
	}

	var smtpAuth smtp.Auth
	if username := os.Getenv("SMTP_USERNAME"); username != "" {
		host, _, err := net.SplitHostPort(smtpAddr)
		if err != nil {
			return nil, fmt.Errorf("invalid SMTP_ADDR: %w", err)
		}
		smtpAuth = smtp.PlainAuth("", username, os.Getenv("SMTP_PASSWORD"), host)
	}

	return mail.NewSMTPSender(smtpAddr, smtpFrom, smtpAuth), nil
}

// familyReminderLeadTime parses FAMILY_REMINDER_LEAD_TIME (a
// time.ParseDuration string, e.g. "24h"), defaulting to 24h.
func familyReminderLeadTime() (time.Duration, error) {
	v := os.Getenv("FAMILY_REMINDER_LEAD_TIME")
	if v == "" {
		return 24 * time.Hour, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("invalid FAMILY_REMINDER_LEAD_TIME: %w", err)
	}
	return d, nil
}

// digestHour is the local hour (0-23) the staff digest fires at daily,
// from DIGEST_HOUR, defaulting to 17 (5pm).
func digestHour() (int, error) {
	v := os.Getenv("DIGEST_HOUR")
	if v == "" {
		return 17, nil
	}
	hour, err := strconv.Atoi(v)
	if err != nil || hour < 0 || hour > 23 {
		return 0, fmt.Errorf("invalid DIGEST_HOUR %q: must be an integer 0-23", v)
	}
	return hour, nil
}

func addr() string {
	if a := os.Getenv("HTTP_ADDR"); a != "" {
		return a
	}
	return ":8080"
}

// secureCookies reports whether the session cookie should be marked Secure
// (HTTPS-only). Defaults to true; set SECURE_COOKIES=false for local
// development over plain HTTP.
func secureCookies() bool {
	return os.Getenv("SECURE_COOKIES") != "false"
}
