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
	"github.com/ggem/coglab-manager-go/internal/mcdi"
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

	mcdiClient, err := newMCDIClient()
	if err != nil {
		return err
	}

	queries := db.New(pool)

	oidcAuthenticator, err := newOIDCAuthenticator(ctx, queries)
	if err != nil {
		return err
	}

	mailer, err := newMailer()
	if err != nil {
		return err
	}
	baseURL, err := appBaseURL()
	if err != nil {
		return err
	}

	server := httpapi.NewServer(
		auth.NewPasswordAuthenticator(queries),
		auth.NewSessionManager(queries, secureCookies()),
		audit.NewRecorder(queries),
		queries,
		pool,
		mcdiClient,
		logger,
		oidcAuthenticator,
		mailer,
		baseURL,
	)

	srv := &http.Server{
		Addr:              addr(),
		Handler:           server.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	leadTime, err := familyReminderLeadTime()
	if err != nil {
		return err
	}
	hour, err := digestHour()
	if err != nil {
		return err
	}
	if err := requireTZ(); err != nil {
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

// newMCDIClient builds the daxlabbase/cdibase client children's "Request
// MCDI" action sends through, from MCDI_API_URL and MCDI_API_KEY, both
// required the same way SMTP_ADDR/SMTP_FROM are -- core functionality
// being wired at startup, not an optional add-on. MCDI_TYPE_PARAM is
// optional: the current cdibase tool's "cdi_type" is mcdi.NewAPIClient's
// own default, so this only needs setting for a still-live older
// daxlabbase instance expecting the pre-rename "mcdi_type".
// newOIDCAuthenticator wires up SSO, if configured. Unlike newMCDIClient
// (always required), OIDC_* is optional -- most deployments start with no
// institutional IdP registered yet, and local password login must keep
// working regardless. But it's all-or-nothing: a partially-set
// configuration (e.g. an issuer URL with no client secret) fails fast at
// startup rather than silently running with SSO half-broken.
//
// Returns the auth.SSOAuthenticator interface, not the concrete
// *auth.OIDCAuthenticator -- returning a typed nil *auth.OIDCAuthenticator
// here and assigning it to Server.oidc (itself an interface field) would
// produce a non-nil interface value wrapping a nil pointer, so
// `s.oidc != nil` would be true even with SSO unconfigured.
func newOIDCAuthenticator(ctx context.Context, queries db.Querier) (auth.SSOAuthenticator, error) {
	cfg := auth.OIDCConfig{
		IssuerURL:    os.Getenv("OIDC_ISSUER_URL"),
		ClientID:     os.Getenv("OIDC_CLIENT_ID"),
		ClientSecret: os.Getenv("OIDC_CLIENT_SECRET"),
		RedirectURL:  os.Getenv("OIDC_REDIRECT_URL"),
	}
	set := cfg.IssuerURL != "" || cfg.ClientID != "" || cfg.ClientSecret != "" || cfg.RedirectURL != ""
	complete := cfg.IssuerURL != "" && cfg.ClientID != "" && cfg.ClientSecret != "" && cfg.RedirectURL != ""
	if !set {
		return nil, nil
	}
	if !complete {
		return nil, fmt.Errorf("OIDC_ISSUER_URL, OIDC_CLIENT_ID, OIDC_CLIENT_SECRET, and OIDC_REDIRECT_URL must all be set together, or none at all")
	}
	return auth.NewOIDCAuthenticator(ctx, cfg, queries)
}

func newMCDIClient() (mcdi.Client, error) {
	apiURL := os.Getenv("MCDI_API_URL")
	if apiURL == "" {
		return nil, fmt.Errorf("MCDI_API_URL environment variable is required")
	}
	apiKey := os.Getenv("MCDI_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("MCDI_API_KEY environment variable is required")
	}
	return mcdi.NewAPIClient(apiURL, apiKey, os.Getenv("MCDI_TYPE_PARAM")), nil
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

// requireTZ fails fast if the process has no explicit, valid timezone,
// rather than silently defaulting to UTC. digestHour's daily-alignment
// calculation and ListAppointmentsDueForReminder's due-date comparison
// both compare process-local "now" directly against appointments.
// schedule_date/schedule_time_start, which are naive columns holding
// the lab's own local wall-clock digits with no timezone attached --
// correct only when the process's own local time already IS the lab's
// local time. This has always been true by coincidence in local dev
// (developer's machine already in the lab's own timezone), but a
// container defaults to UTC unless told otherwise: caught live when
// that silently misclassified genuinely-future appointments as already
// past.
//
// time.LoadLocation(tz) is a real, if imperfect, validation: it fails
// exactly the same lookup Go's own runtime does to resolve TZ into
// time.Local (same tzdata database, installed in the api image
// specifically so this lookup -- and TZ itself -- works at all), so a
// typo'd or nonexistent zone name is caught here rather than silently
// falling back to UTC the same way an entirely-unset TZ would. It can't
// verify the *correct* zone was chosen (matching the lab's actual
// location) -- that's still a deployment concern, not something a
// syntax check can catch. Also handles TZ="" explicitly: LoadLocation("")
// is specced to succeed as an alias for UTC, which would otherwise let
// an unset-in-practice TZ slip past this check silently, defeating it.
func requireTZ() error {
	tz := os.Getenv("TZ")
	if tz == "" {
		return fmt.Errorf("TZ environment variable is required (e.g. America/Denver, matching the lab's local timezone) -- see requireTZ's doc comment for why")
	}
	if _, err := time.LoadLocation(tz); err != nil {
		return fmt.Errorf("invalid TZ %q: %w", tz, err)
	}
	return nil
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

// appBaseURL is the frontend's public origin, required the same way
// SMTP_ADDR/SMTP_FROM are -- it's the only way httpapi's account-creation
// invite email knows what to build a /set-password?token=... link against,
// since the API itself has no idea what origin the frontend is served
// from.
func appBaseURL() (string, error) {
	url := os.Getenv("APP_BASE_URL")
	if url == "" {
		return "", fmt.Errorf("APP_BASE_URL environment variable is required")
	}
	return url, nil
}
