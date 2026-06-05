package mail

import (
	"context"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/linkasu/linka.looks-metric/internal/config"
)

func TestSendHTMLUnavailableSMTPReturnsError(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}

	mailer := NewSMTPMailer(config.MailConfig{
		Host:      "127.0.0.1",
		Port:      port,
		User:      "user",
		Password:  "password",
		FromEmail: "apps@linka.su",
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := mailer.SendHTML(ctx, "ivan@aacidov.ru", "subject", "<p>body</p>"); err == nil {
		t.Fatal("expected error for unavailable SMTP server")
	}
}
