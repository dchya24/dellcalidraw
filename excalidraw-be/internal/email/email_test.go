package email

import (
	"context"
	"strings"
	"testing"
)

func TestNewFallsBackToLogWhenSMTPMisconfigured(t *testing.T) {
	for _, tc := range []struct {
		name string
		cfg  Config
	}{
		{"empty provider", Config{}},
		{"explicit log", Config{Provider: "log"}},
		{"smtp missing host", Config{Provider: "smtp", From: "from@x"}},
		{"smtp missing from", Config{Provider: "smtp", SMTP: SMTPConfig{Host: "smtp.x"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := New(tc.cfg)
			if _, ok := s.(*LogSender); !ok {
				t.Errorf("expected LogSender, got %T", s)
			}
		})
	}
}

func TestNewReturnsSMTPSenderWhenConfigured(t *testing.T) {
	cfg := Config{
		Provider: "smtp",
		From:     "Excalidraw <noreply@example.com>",
		SMTP:     SMTPConfig{Host: "smtp.example.com"},
	}
	s := New(cfg)
	if _, ok := s.(*SMTPSender); !ok {
		t.Fatalf("expected SMTPSender, got %T", s)
	}
}

func TestLogSenderSendIsBenign(t *testing.T) {
	s := &LogSender{Reason: "tests"}
	err := s.Send(context.Background(), Message{
		To:        "u@x",
		Subject:   "Test",
		PlainText: "hi",
	})
	if err != nil {
		t.Fatalf("LogSender.Send returned error: %v", err)
	}
}

func TestPasswordResetMessageContents(t *testing.T) {
	url := "http://example.com/reset-password?token=abc123"
	msg := PasswordResetMessage("user@example.com", url, 60)

	if msg.To != "user@example.com" {
		t.Errorf("To: got %q", msg.To)
	}
	if !strings.Contains(strings.ToLower(msg.Subject), "reset") {
		t.Errorf("Subject should mention reset: %q", msg.Subject)
	}
	if !strings.Contains(msg.PlainText, url) {
		t.Errorf("plaintext must include the reset URL")
	}
	if !strings.Contains(msg.PlainText, "60") {
		t.Errorf("plaintext should mention 60-minute expiry")
	}
	if !strings.Contains(msg.HTML, "Reset password") {
		t.Errorf("HTML should contain the call-to-action label")
	}
	if !strings.Contains(msg.HTML, url) {
		t.Errorf("HTML must include the reset URL")
	}
}

func TestPasswordResetMessageEscapesURL(t *testing.T) {
	// HTML escape protects against XSS if a malicious base URL ever
	// reaches the template. Plaintext deliberately doesn't escape.
	url := `http://example.com/reset?x=<script>alert(1)</script>`
	msg := PasswordResetMessage("u@x", url, 60)

	if strings.Contains(msg.HTML, "<script>") {
		t.Fatal("HTML contains unescaped <script> tag from URL")
	}
	if !strings.Contains(msg.PlainText, "<script>") {
		t.Fatal("plaintext should keep the URL verbatim")
	}
}

func TestRoomInvitationMessageDefaults(t *testing.T) {
	msg := RoomInvitationMessage("guest@example.com", "", "", "http://app/room?x=y")

	if !strings.Contains(msg.PlainText, "Someone") {
		t.Errorf("missing inviter fallback")
	}
	if !strings.Contains(msg.PlainText, "Excalidraw room") {
		t.Errorf("missing room name fallback")
	}
}

func TestExtractAddr(t *testing.T) {
	cases := map[string]string{
		"plain@example.com":               "plain@example.com",
		"Name <user@example.com>":         "user@example.com",
		"  Name <user@example.com>  ":     "user@example.com",
		"\"Quoted Name\" <q@example.com>": "q@example.com",
	}
	for in, want := range cases {
		if got := extractAddr(in); got != want {
			t.Errorf("extractAddr(%q) = %q; want %q", in, got, want)
		}
	}
}

func TestBuildMIMEPlainTextOnly(t *testing.T) {
	out := buildMIME("from@x", Message{
		To:        "to@x",
		Subject:   "S",
		PlainText: "hello",
	})
	if !strings.Contains(out, "Content-Type: text/plain") {
		t.Errorf("expected text/plain content type:\n%s", out)
	}
	if strings.Contains(out, "multipart/alternative") {
		t.Errorf("plaintext-only message should not be multipart")
	}
	if !strings.Contains(out, "From: from@x\r\n") {
		t.Errorf("missing From header")
	}
	if !strings.Contains(out, "To: to@x\r\n") {
		t.Errorf("missing To header")
	}
	if !strings.Contains(out, "Subject: S\r\n") {
		t.Errorf("missing Subject header")
	}
}

func TestBuildMIMEMultipart(t *testing.T) {
	out := buildMIME("from@x", Message{
		To:        "to@x",
		Subject:   "S",
		PlainText: "plain body",
		HTML:      "<p>html body</p>",
	})

	for _, want := range []string{
		"multipart/alternative; boundary=",
		"text/plain; charset=UTF-8",
		"text/html; charset=UTF-8",
		"plain body",
		"<p>html body</p>",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in MIME body", want)
		}
	}
}

func TestSMTPSenderRejectsBadMessages(t *testing.T) {
	s := NewSMTPSender(Config{From: "from@x", SMTP: SMTPConfig{Host: "h"}})
	for name, msg := range map[string]Message{
		"missing To":      {Subject: "s", PlainText: "p"},
		"missing Subject": {To: "t@x", PlainText: "p"},
		"missing Body":    {To: "t@x", Subject: "s"},
	} {
		t.Run(name, func(t *testing.T) {
			err := s.Send(context.Background(), msg)
			if err == nil {
				t.Errorf("expected error for %s", name)
			}
		})
	}
}
