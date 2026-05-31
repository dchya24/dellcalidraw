// Package email sends transactional email (password reset, invitations).
//
// The package exposes a Sender interface so the application can swap
// implementations based on EMAIL_PROVIDER:
//
//   "log"   — write the message to slog. Dev fallback. Default when
//             nothing is configured.
//   "smtp"  — net/smtp with PLAIN auth + STARTTLS. Works against any
//             standards-compliant relay (Gmail, Postmark, SES SMTP,
//             Mailgun, self-hosted, etc.).
//
// Templates live in templates.go and are kept inline + small. Bigger
// HTML payloads belong in `embed.FS` once we have more than two
// transactional emails.
package email

import (
	"context"
	"errors"
)

// Message describes an outbound email. PlainText is required (RFC
// fallback for clients that don't render HTML); HTML is optional.
type Message struct {
	To       string
	Subject  string
	HTML     string
	PlainText string
}

// Sender is the application-facing API. Implementations must be
// safe for concurrent use across requests.
type Sender interface {
	Send(ctx context.Context, msg Message) error
}

// ErrNotConfigured signals that the caller asked for a provider that
// isn't configured (e.g. SMTP requested without a host). Handlers can
// check for this to decide whether to fall back gracefully or fail.
var ErrNotConfigured = errors.New("email provider not configured")

// Config carries the settings the package consumes from the
// application config. The application maps env vars or YAML keys
// onto this struct.
type Config struct {
	Provider string // "log" (default) | "smtp"
	From     string // RFC-5322 mailbox, e.g. "Excalidraw <no-reply@example.com>"
	BaseURL  string // App base URL used to build links inside emails

	SMTP SMTPConfig
}

// SMTPConfig configures the SMTP sender.
type SMTPConfig struct {
	Host     string // e.g. "smtp.postmarkapp.com"
	Port     int    // e.g. 587
	Username string
	Password string
	// UseSTARTTLS upgrades the connection after EHLO. Default true.
	// Set false for plaintext relays in trusted networks (rare).
	UseSTARTTLS bool
}

// New returns a Sender for the configured provider. When Provider is
// empty or "log", returns a LogSender that prints messages via slog.
// When Provider is "smtp" but the SMTP config is incomplete, returns
// the LogSender with a warning so dev environments don't crash.
func New(cfg Config) Sender {
	switch cfg.Provider {
	case "smtp":
		if cfg.SMTP.Host == "" || cfg.From == "" {
			return &LogSender{Reason: "smtp configured without Host/From; falling back to log"}
		}
		return NewSMTPSender(cfg)
	default:
		return &LogSender{}
	}
}
