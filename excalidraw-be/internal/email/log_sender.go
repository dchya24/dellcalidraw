package email

import (
	"context"
	"log/slog"
)

// LogSender writes outbound mail to the application logger instead
// of delivering it. This is the dev default: it lets engineers see
// the reset URL without wiring up an SMTP relay.
//
// Reason, when non-empty, is logged once at construction time so
// it's clear *why* we're falling back (e.g. SMTP misconfigured).
type LogSender struct {
	Reason string
	logged bool
}

func (l *LogSender) Send(_ context.Context, msg Message) error {
	if !l.logged && l.Reason != "" {
		slog.Warn("[email] using LogSender", "reason", l.Reason)
		l.logged = true
	}
	slog.Info("[email] would send",
		"to", msg.To,
		"subject", msg.Subject,
		// PlainText is logged at debug to avoid leaking reset URLs
		// into structured production logs by accident. Operators
		// can flip log.level=debug temporarily when investigating.
	)
	slog.Debug("[email] body",
		"to", msg.To,
		"plainText", msg.PlainText,
	)
	return nil
}
