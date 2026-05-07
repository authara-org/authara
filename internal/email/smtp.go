package email

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/mail"
	"net/smtp"
	"strings"
	"time"
)

type SMTPSender struct {
	host     string
	port     int
	username string
	password string
	from     string
	useTLS   bool
	timeout  time.Duration
}

func NewSMTPSender(
	host string,
	port int,
	username string,
	password string,
	from string,
	useTLS bool,
	timeout time.Duration,
) *SMTPSender {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	return &SMTPSender{
		host:     host,
		port:     port,
		username: username,
		password: password,
		from:     from,
		useTLS:   useTLS,
		timeout:  timeout,
	}
}

func (s *SMTPSender) Send(ctx context.Context, to string, msg Message) error {
	fromAddr, err := parseEnvelopeAddress("from", s.from)
	if err != nil {
		return err
	}
	toAddr, err := parseEnvelopeAddress("to", to)
	if err != nil {
		return err
	}

	raw, err := buildMIMEMessage(s.from, to, msg)
	if err != nil {
		return fmt.Errorf("smtp: build message: %w", err)
	}

	addr := fmt.Sprintf("%s:%d", s.host, s.port)

	dialer := net.Dialer{Timeout: s.timeout}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("smtp: dial: %w", err)
	}
	defer conn.Close()

	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	} else {
		_ = conn.SetDeadline(time.Now().Add(s.timeout))
	}

	client, err := smtp.NewClient(conn, s.host)
	if err != nil {
		return fmt.Errorf("smtp: client: %w", err)
	}
	defer client.Close()

	if s.useTLS {
		tlsConfig := &tls.Config{
			ServerName: s.host,
			MinVersion: tls.VersionTLS12,
		}

		if ok, _ := client.Extension("STARTTLS"); !ok {
			return fmt.Errorf("smtp: server does not support STARTTLS")
		}

		if err := client.StartTLS(tlsConfig); err != nil {
			return fmt.Errorf("smtp: starttls: %w", err)
		}
	}

	if s.username != "" && s.password != "" {
		auth := smtp.PlainAuth("", s.username, s.password, s.host)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("smtp: auth: %w", err)
		}
	}

	if err := client.Mail(fromAddr); err != nil {
		return fmt.Errorf("smtp: mail: %w", err)
	}
	if err := client.Rcpt(toAddr); err != nil {
		return fmt.Errorf("smtp: rcpt: %w", err)
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp: data: %w", err)
	}

	if _, err := w.Write(raw); err != nil {
		_ = w.Close()
		return fmt.Errorf("smtp: write: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("smtp: close: %w", err)
	}

	if err := client.Quit(); err != nil {
		return fmt.Errorf("smtp: quit: %w", err)
	}

	return nil
}

func buildMIMEMessage(from, to string, msg Message) ([]byte, error) {
	if msg.Text == "" && msg.HTML == "" {
		return nil, fmt.Errorf("smtp: email message must contain text or html body")
	}
	fromAddr, err := parseHeaderAddress("From", from)
	if err != nil {
		return nil, err
	}
	toAddr, err := parseHeaderAddress("To", to)
	if err != nil {
		return nil, err
	}
	if err := validateHeaderValue("Subject", msg.Subject); err != nil {
		return nil, err
	}

	boundary := fmt.Sprintf("authara-%d", time.Now().UnixNano())

	var b strings.Builder

	writeHeader(&b, "From", fromAddr)
	writeHeader(&b, "To", toAddr)
	writeHeader(&b, "Subject", msg.Subject)
	writeHeader(&b, "MIME-Version", "1.0")
	writeHeader(&b, "Content-Type", fmt.Sprintf(`multipart/alternative; boundary="%s"`, boundary))
	b.WriteString("\r\n")

	if msg.Text != "" {
		fmt.Fprintf(&b, "--%s\r\n", boundary)
		writeHeader(&b, "Content-Type", `text/plain; charset="UTF-8"`)
		writeHeader(&b, "Content-Transfer-Encoding", "8bit")
		b.WriteString("\r\n")
		b.WriteString(normalizeSMTPBody(msg.Text))
		b.WriteString("\r\n")
	}

	if msg.HTML != "" {
		fmt.Fprintf(&b, "--%s\r\n", boundary)
		writeHeader(&b, "Content-Type", `text/html; charset="UTF-8"`)
		writeHeader(&b, "Content-Transfer-Encoding", "8bit")
		b.WriteString("\r\n")
		b.WriteString(normalizeSMTPBody(msg.HTML))
		b.WriteString("\r\n")
	}

	b.WriteString(fmt.Sprintf("--%s--\r\n", boundary))

	return []byte(b.String()), nil
}

func writeHeader(b *strings.Builder, key, value string) {
	b.WriteString(key)
	b.WriteString(": ")
	b.WriteString(value)
	b.WriteString("\r\n")
}

func parseEnvelopeAddress(label string, raw string) (string, error) {
	addr, err := mail.ParseAddress(raw)
	if err != nil {
		return "", fmt.Errorf("smtp: invalid %s address: %w", label, err)
	}
	return addr.Address, nil
}

func parseHeaderAddress(key string, raw string) (string, error) {
	if err := validateHeaderValue(key, raw); err != nil {
		return "", err
	}
	addr, err := mail.ParseAddress(raw)
	if err != nil {
		return "", fmt.Errorf("smtp: invalid %s address: %w", strings.ToLower(key), err)
	}
	return addr.String(), nil
}

func validateHeaderValue(key string, value string) error {
	if strings.ContainsAny(value, "\r\n") {
		return fmt.Errorf("smtp: %s header contains line breaks", key)
	}
	return nil
}

func normalizeSMTPBody(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return strings.ReplaceAll(s, "\n", "\r\n")
}
