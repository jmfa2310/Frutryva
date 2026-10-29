package identity

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/textproto"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestDevelopmentSMTPRejectsRemoteTransport(t *testing.T) {
	for _, host := range []string{"smtp.example.invalid", "mailpit.example.invalid", "127.0.0.1.example.invalid", ""} {
		if _, err := NewSMTPMailer(host, 1025); err == nil {
			t.Errorf("insecure remote development SMTP accepted: %q", host)
		}
	}
	if _, err := NewSMTPMailer("mailpit", 0); err == nil {
		t.Fatal("invalid port accepted")
	}
	mailer, err := NewSMTPMailer("mailpit", 1025)
	if err != nil {
		t.Fatal(err)
	}
	if err := mailer.Send(context.Background(), "bad\r\nBcc: victim@example.invalid", "subject", "body"); err == nil {
		t.Fatal("header injection accepted")
	}
	if err := mailer.Send(context.Background(), "synthetic@example.invalid", "bad\r\nheader", "body"); err == nil {
		t.Fatal("subject injection accepted")
	}
}

func TestDevelopmentSMTPActuallyDeliversMessage(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	type delivery struct {
		message string
		err     error
	}
	result := make(chan delivery, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			result <- delivery{err: err}
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
		protocol := textproto.NewConn(conn)
		if err := protocol.PrintfLine("220 local test mailbox"); err != nil {
			result <- delivery{err: err}
			return
		}
		var message string
		for {
			line, err := protocol.ReadLine()
			if err != nil {
				result <- delivery{err: err}
				return
			}
			switch {
			case strings.HasPrefix(line, "EHLO "):
				err = protocol.PrintfLine("250-localhost\r\n250 8BITMIME")
			case strings.HasPrefix(line, "MAIL FROM:"), strings.HasPrefix(line, "RCPT TO:"):
				err = protocol.PrintfLine("250 accepted")
			case line == "DATA":
				if err = protocol.PrintfLine("354 send message"); err != nil {
					result <- delivery{err: err}
					return
				}
				data, readErr := io.ReadAll(protocol.DotReader())
				if readErr != nil {
					result <- delivery{err: readErr}
					return
				}
				message = string(data)
				err = protocol.PrintfLine("250 queued")
			case line == "QUIT":
				err = protocol.PrintfLine("221 goodbye")
				result <- delivery{message: message, err: err}
				return
			default:
				result <- delivery{err: fmt.Errorf("unexpected SMTP command %q", line)}
				return
			}
			if err != nil {
				result <- delivery{err: err}
				return
			}
		}
	}()
	_, portText, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatal(err)
	}
	mailer, err := NewSMTPMailer("127.0.0.1", port)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	if err := mailer.Send(ctx, "synthetic@example.invalid", "Confirmar email", "Synthetic one-time code\r\n"); err != nil {
		t.Fatal(err)
	}
	delivered := <-result
	if delivered.err != nil {
		t.Fatal(delivered.err)
	}
	if !strings.Contains(delivered.message, "To: <synthetic@example.invalid>") || !strings.Contains(delivered.message, "Synthetic one-time code") || !strings.Contains(delivered.message, "Content-Type: text/plain; charset=UTF-8") {
		t.Fatal("SMTP did not deliver expected message headers/body")
	}
}
