package mail

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"net/smtp"
	"strings"
	"time"

	"github.com/linkasu/linka.looks-metric/internal/config"
)

type Mailer interface {
	SendActivationCode(ctx context.Context, to string, code string) error
}

type SMTPMailer struct {
	cfg    config.MailConfig
	logger *slog.Logger
}

func NewSMTPMailer(cfg config.MailConfig, logger *slog.Logger) *SMTPMailer {
	return &SMTPMailer{cfg: cfg, logger: logger}
}

func (m *SMTPMailer) SendActivationCode(ctx context.Context, to string, code string) error {
	subject := "Код активации для LINKa смотри. " + code
	body := fmt.Sprintf(`<p>Ваш код активации:</p><p style="font-size:24px;font-weight:700">%s</p>`, code)
	return m.SendHTML(ctx, to, subject, body)
}

func (m *SMTPMailer) SendHTML(ctx context.Context, to string, subject string, html string) error {
	if m.cfg.DryRun {
		m.logger.Info("mail dry run", "to", to, "subject", subject)
		return nil
	}

	done := make(chan error, 1)
	go func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				done <- fmt.Errorf("send mail panic: %v", recovered)
			}
		}()
		done <- m.sendHTML(to, subject, html)
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-done:
		return err
	}
}

func (m *SMTPMailer) sendHTML(to string, subject string, html string) error {
	addr := fmt.Sprintf("%s:%d", m.cfg.Host, m.cfg.Port)
	dialer := &net.Dialer{Timeout: 20 * time.Second}
	conn, err := tls.DialWithDialer(dialer, "tcp", addr, &tls.Config{
		ServerName: m.cfg.Host,
		MinVersion: tls.VersionTLS12,
	})
	if err != nil {
		return err
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(20 * time.Second))

	client, err := smtp.NewClient(conn, m.cfg.Host)
	if err != nil {
		return err
	}
	defer client.Quit()

	if err := client.Auth(smtp.PlainAuth("", m.cfg.User, m.cfg.Password, m.cfg.Host)); err != nil {
		return err
	}
	from := m.cfg.FromEmail
	if err := client.Mail(from); err != nil {
		return err
	}
	if err := client.Rcpt(to); err != nil {
		return err
	}
	writer, err := client.Data()
	if err != nil {
		return err
	}
	message := strings.Join([]string{
		"From: " + formatAddress(m.cfg.FromName, from),
		"To: " + to,
		"Subject: " + subject,
		"MIME-Version: 1.0",
		"Content-Type: text/html; charset=UTF-8",
		"",
		html,
	}, "\r\n")
	if _, err := writer.Write([]byte(message)); err != nil {
		writer.Close()
		return err
	}
	return writer.Close()
}

func formatAddress(name string, email string) string {
	if strings.TrimSpace(name) == "" {
		return email
	}
	return fmt.Sprintf(`"%s" <%s>`, strings.ReplaceAll(name, `"`, `\"`), email)
}
