package email

import (
	"fmt"
	"html"
	"strings"
)

// PasswordResetMessage builds a transactional Message for the
// "you requested a password reset" flow. resetURL is the full URL
// the user clicks. expiryMinutes is how long the token is valid.
//
// The HTML uses inline styles only (most clients strip <style>
// blocks). Keep it minimal — branding belongs in templates that
// land via embed.FS once we have more transactional emails.
func PasswordResetMessage(toEmail, resetURL string, expiryMinutes int) Message {
	subject := "Reset your Excalidraw password"

	plain := fmt.Sprintf(`Hi,

We received a request to reset the password for your Excalidraw account.

Click the link below to set a new password. The link will expire in %d minutes.

%s

If you didn't request this, you can safely ignore this email — your
password won't change.

— Excalidraw
`, expiryMinutes, resetURL)

	htmlBody := fmt.Sprintf(`<!DOCTYPE html>
<html>
  <body style="margin:0;padding:0;background:#f5f5f5;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif;">
    <div style="max-width:520px;margin:32px auto;padding:24px;background:#ffffff;border-radius:8px;border:1px solid #e5e7eb;">
      <h1 style="font-size:18px;margin:0 0 16px 0;color:#111827;">Reset your password</h1>
      <p style="font-size:14px;color:#374151;line-height:1.6;">
        We received a request to reset the password for your Excalidraw account.
      </p>
      <p style="font-size:14px;color:#374151;line-height:1.6;">
        Click the button below to set a new password. The link will expire in %d minutes.
      </p>
      <p style="margin:24px 0;">
        <a href="%s" style="display:inline-block;padding:10px 16px;background:#4a9eed;color:#ffffff;text-decoration:none;border-radius:6px;font-size:14px;font-weight:500;">Reset password</a>
      </p>
      <p style="font-size:12px;color:#6b7280;line-height:1.6;">
        Or copy and paste this URL into your browser:<br/>
        <span style="word-break:break-all;">%s</span>
      </p>
      <p style="font-size:12px;color:#9ca3af;margin-top:24px;line-height:1.6;">
        If you didn't request this, you can safely ignore this email —
        your password won't change.
      </p>
    </div>
  </body>
</html>`, expiryMinutes, html.EscapeString(resetURL), html.EscapeString(resetURL))

	return Message{
		To:        toEmail,
		Subject:   subject,
		HTML:      strings.TrimSpace(htmlBody),
		PlainText: plain,
	}
}

// RoomInvitationMessage builds the email for a room collaboration
// invite. inviteURL is what the recipient clicks; inviterName +
// roomName are interpolated into the body. Reused for #15.
func RoomInvitationMessage(toEmail, inviterName, roomName, inviteURL string) Message {
	if roomName == "" {
		roomName = "an Excalidraw room"
	}
	if inviterName == "" {
		inviterName = "Someone"
	}

	subject := fmt.Sprintf("%s invited you to %s", inviterName, roomName)

	plain := fmt.Sprintf(`Hi,

%s invited you to collaborate on %q on Excalidraw.

Open the room here:

%s

If you weren't expecting this invitation you can ignore this email.

— Excalidraw
`, inviterName, roomName, inviteURL)

	htmlBody := fmt.Sprintf(`<!DOCTYPE html>
<html>
  <body style="margin:0;padding:0;background:#f5f5f5;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif;">
    <div style="max-width:520px;margin:32px auto;padding:24px;background:#ffffff;border-radius:8px;border:1px solid #e5e7eb;">
      <h1 style="font-size:18px;margin:0 0 16px 0;color:#111827;">You've been invited</h1>
      <p style="font-size:14px;color:#374151;line-height:1.6;">
        <strong>%s</strong> invited you to collaborate on
        <strong>%s</strong> on Excalidraw.
      </p>
      <p style="margin:24px 0;">
        <a href="%s" style="display:inline-block;padding:10px 16px;background:#4a9eed;color:#ffffff;text-decoration:none;border-radius:6px;font-size:14px;font-weight:500;">Open room</a>
      </p>
      <p style="font-size:12px;color:#6b7280;line-height:1.6;">
        Or copy this URL into your browser:<br/>
        <span style="word-break:break-all;">%s</span>
      </p>
    </div>
  </body>
</html>`, html.EscapeString(inviterName), html.EscapeString(roomName), html.EscapeString(inviteURL), html.EscapeString(inviteURL))

	return Message{
		To:        toEmail,
		Subject:   subject,
		HTML:      strings.TrimSpace(htmlBody),
		PlainText: plain,
	}
}
