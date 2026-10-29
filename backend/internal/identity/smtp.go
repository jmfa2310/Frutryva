package identity

import (
	"context"
	"errors"
	"fmt"
	"mime"
	"net"
	"net/mail"
	"net/smtp"
	"strconv"
	"strings"
	"time"
)

// SMTPMailer is intentionally limited to the local development inbox. Production
// delivery requires a separately configured, authenticated TLS mail transport.
type SMTPMailer struct {
	host string
	port int
}

func NewSMTPMailer(host string, port int) (*SMTPMailer, error) {
	if host != "mailpit" && host != "localhost" && host != "127.0.0.1" && host != "::1" {
		return nil, errors.New("development SMTP host must be mailpit or loopback")
	}
	if port < 1 || port > 65535 {
		return nil, errors.New("invalid SMTP port")
	}
	return &SMTPMailer{host: host, port: port}, nil
}

func (m *SMTPMailer) Send(ctx context.Context, to, subject, body string) error {
	if _, err := normalizedEmail(to); err != nil {
		return errors.New("invalid mail recipient")
	}
	if strings.ContainsAny(subject, "\r\n") {
		return errors.New("invalid mail subject")
	}
	address := net.JoinHostPort(m.host, strconv.Itoa(m.port))
	dialer := net.Dialer{Timeout: 3 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return errors.New("development mailbox unavailable")
	}
	defer conn.Close()
	deadline := time.Now().Add(5 * time.Second)
	if parent, ok := ctx.Deadline(); ok && parent.Before(deadline) {
		deadline = parent
	}
	if err := conn.SetDeadline(deadline); err != nil {
		return errors.New("cannot configure SMTP connection")
	}
	client, err := smtp.NewClient(conn, m.host)
	if err != nil {
		return errors.New("SMTP greeting failed")
	}
	defer client.Close()
	if err := client.Mail("no-reply@futryva.local"); err != nil {
		return errors.New("SMTP sender rejected")
	}
	if err := client.Rcpt(to); err != nil {
		return errors.New("SMTP recipient rejected")
	}
	writer, err := client.Data()
	if err != nil {
		return errors.New("SMTP message rejected")
	}
	recipient := (&mail.Address{Address: to}).String()
	message := "From: FUTRYVA <no-reply@futryva.local>\r\nTo: " + recipient + "\r\nSubject: " + mime.QEncoding.Encode("UTF-8", subject) + "\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\nContent-Transfer-Encoding: 8bit\r\n\r\n" + body
	if _, err := fmt.Fprint(writer, message); err != nil {
		return errors.New("SMTP message transfer failed")
	}
	if err := writer.Close(); err != nil {
		return errors.New("SMTP message was not accepted")
	}
	if err := client.Quit(); err != nil {
		return errors.New("SMTP session did not complete")
	}
	return nil
}
