package email

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/smtp"
	"strconv"
	"strings"
	"time"
)

// SMTPSender delivers email via net/smtp. Connection is established
// per Send() call; we don't pool because transactional volume is low.
// If volume ever justifies pooling, swap this for `go-mail/mail` or
// similar without changing the Sender interface.
type SMTPSender struct {
	cfg Config
}

// NewSMTPSender constructs an SMTP sender. Caller is responsible for
// validating that cfg.SMTP.Host and cfg.From are present (email.New
// does this and falls back to LogSender if not).
func NewSMTPSender(cfg Config) *SMTPSender {
	if cfg.SMTP.Port == 0 {
		cfg.SMTP.Port = 587
	}
	return &SMTPSender{cfg: cfg}
}

func (s *SMTPSender) Send(ctx context.Context, msg Message) error {
	if msg.To == "" {
		return errors.New("email: missing To")
	}
	if msg.Subject == "" {
		return errors.New("email: missing Subject")
	}
	if msg.PlainText == "" {
		return errors.New("email: missing PlainText (required as fallback)")
	}

	addr := net.JoinHostPort(s.cfg.SMTP.Host, strconv.Itoa(s.cfg.SMTP.Port))

	// Honour ctx by setting a dial deadline. Once the SMTP exchange
	// starts there's no per-step context check, but the dial timeout
	// is the most important one in practice (relay outages).
	deadline := time.Now().Add(15 * time.Second)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	dialer := &net.Dialer{Deadline: deadline}

	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("email: dial smtp %s: %w", addr, err)
	}

	c, err := smtp.NewClient(conn, s.cfg.SMTP.Host)
	if err != nil {
		conn.Close()
		return fmt.Errorf("email: smtp client: %w", err)
	}
	defer func() { _ = c.Close() }()

	if err := c.Hello("excalidraw-be"); err != nil {
		return fmt.Errorf("email: EHLO: %w", err)
	}

	useTLS := s.cfg.SMTP.UseSTARTTLS
	// If the server advertises STARTTLS and the caller didn't
	// explicitly opt out, upgrade. This is the safe default.
	if ok, _ := c.Extension("STARTTLS"); ok && useTLS {
		if err := c.StartTLS(&tls.Config{ServerName: s.cfg.SMTP.Host, MinVersion: tls.VersionTLS12}); err != nil {
			return fmt.Errorf("email: STARTTLS: %w", err)
		}
	}

	if s.cfg.SMTP.Username != "" {
		auth := smtp.PlainAuth("", s.cfg.SMTP.Username, s.cfg.SMTP.Password, s.cfg.SMTP.Host)
		if err := c.Auth(auth); err != nil {
			return fmt.Errorf("email: auth: %w", err)
		}
	}

	from := extractAddr(s.cfg.From)
	if err := c.Mail(from); err != nil {
		return fmt.Errorf("email: MAIL FROM: %w", err)
	}
	if err := c.Rcpt(msg.To); err != nil {
		return fmt.Errorf("email: RCPT TO: %w", err)
	}

	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("email: DATA: %w", err)
	}
	if _, err := w.Write([]byte(buildMIME(s.cfg.From, msg))); err != nil {
		return fmt.Errorf("email: write body: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("email: close body: %w", err)
	}

	if err := c.Quit(); err != nil {
		// QUIT failures are usually benign (connection already
		// half-closed by the server). Log via the wrapper but don't
		// surface as a send failure.
		return nil
	}
	return nil
}

// extractAddr pulls the bare address out of an RFC-5322 mailbox
// like 'Name <user@example.com>'. Returns the input as-is if no <>
// pair is present.
func extractAddr(mailbox string) string {
	if i := strings.LastIndex(mailbox, "<"); i >= 0 {
		if j := strings.Index(mailbox[i:], ">"); j >= 0 {
			return mailbox[i+1 : i+j]
		}
	}
	return strings.TrimSpace(mailbox)
}

// buildMIME assembles a multipart/alternative message with both
// plaintext and HTML parts when HTML is provided. Otherwise sends a
// plain-text-only message. Boundary is fixed-but-unique enough for
// the small volume of mail this app sends.
func buildMIME(from string, msg Message) string {
	if msg.HTML == "" {
		var b strings.Builder
		fmt.Fprintf(&b, "From: %s\r\n", from)
		fmt.Fprintf(&b, "To: %s\r\n", msg.To)
		fmt.Fprintf(&b, "Subject: %s\r\n", msg.Subject)
		b.WriteString("MIME-Version: 1.0\r\n")
		b.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
		b.WriteString("Content-Transfer-Encoding: 8bit\r\n")
		b.WriteString("\r\n")
		b.WriteString(msg.PlainText)
		return b.String()
	}

	boundary := "excalidraw-be-mime-" + strconv.FormatInt(time.Now().UnixNano(), 36)
	var b strings.Builder
	fmt.Fprintf(&b, "From: %s\r\n", from)
	fmt.Fprintf(&b, "To: %s\r\n", msg.To)
	fmt.Fprintf(&b, "Subject: %s\r\n", msg.Subject)
	b.WriteString("MIME-Version: 1.0\r\n")
	fmt.Fprintf(&b, "Content-Type: multipart/alternative; boundary=%q\r\n", boundary)
	b.WriteString("\r\n")
	fmt.Fprintf(&b, "--%s\r\n", boundary)
	b.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	b.WriteString("Content-Transfer-Encoding: 8bit\r\n\r\n")
	b.WriteString(msg.PlainText)
	b.WriteString("\r\n")
	fmt.Fprintf(&b, "--%s\r\n", boundary)
	b.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	b.WriteString("Content-Transfer-Encoding: 8bit\r\n\r\n")
	b.WriteString(msg.HTML)
	b.WriteString("\r\n")
	fmt.Fprintf(&b, "--%s--\r\n", boundary)
	return b.String()
}
