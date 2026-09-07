package email

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"strconv"
	"strings"
	"time"
)

// SMTPProvider sends mail via net/smtp with optional STARTTLS (typical port 587)
// or implicit TLS on port 465.
type SMTPProvider struct {
	Host     string
	Port     string
	User     string
	Password string
	From     string
}

func (p *SMTPProvider) Send(ctx context.Context, to, subject, body string) error {
	host := strings.TrimSpace(p.Host)
	if host == "" {
		return fmt.Errorf("email: smtp host is empty")
	}
	port := strings.TrimSpace(p.Port)
	if port == "" {
		port = "587"
	}
	from := strings.TrimSpace(p.From)
	if from == "" {
		return fmt.Errorf("email: smtp from address is empty")
	}
	addr := net.JoinHostPort(host, port)

	msg := []byte(fmt.Sprintf(
		"To: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n%s",
		to, subject, body,
	))

	d := net.Dialer{Timeout: 15 * time.Second}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("email: smtp dial: %w", err)
	}
	defer conn.Close()

	if implicitSMTPTLS(port) {
		tlsConn := tls.Client(conn, &tls.Config{ServerName: host})
		if err := tlsConn.HandshakeContext(ctx); err != nil {
			return fmt.Errorf("email: smtp tls handshake: %w", err)
		}
		conn = tlsConn
	}

	c, err := smtp.NewClient(conn, host)
	if err != nil {
		return fmt.Errorf("email: smtp client: %w", err)
	}
	defer c.Close()

	if !implicitSMTPTLS(port) {
		if ok, _ := c.Extension("STARTTLS"); ok {
			tcfg := &tls.Config{ServerName: host}
			if err := c.StartTLS(tcfg); err != nil {
				return fmt.Errorf("email: smtp starttls: %w", err)
			}
		}
	}

	if p.User != "" {
		a := smtp.PlainAuth("", p.User, p.Password, host)
		if err := c.Auth(a); err != nil {
			return fmt.Errorf("email: smtp auth: %w", err)
		}
	}

	if err := c.Mail(from); err != nil {
		return fmt.Errorf("email: smtp mail: %w", err)
	}
	if err := c.Rcpt(to); err != nil {
		return fmt.Errorf("email: smtp rcpt: %w", err)
	}
	wc, err := c.Data()
	if err != nil {
		return fmt.Errorf("email: smtp data: %w", err)
	}
	if _, err := wc.Write(msg); err != nil {
		_ = wc.Close()
		return fmt.Errorf("email: smtp write body: %w", err)
	}
	if err := wc.Close(); err != nil {
		return fmt.Errorf("email: smtp close data: %w", err)
	}
	if err := c.Quit(); err != nil {
		return fmt.Errorf("email: smtp quit: %w", err)
	}
	return nil
}

func implicitSMTPTLS(port string) bool {
	n, err := strconv.Atoi(port)
	if err != nil {
		return false
	}
	return n == 465
}
