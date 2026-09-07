package email

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

func runTestSMTPServer(t *testing.T) (host string, port string, cleanup func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		c, err := ln.Accept()
		if err != nil {
			return
		}
		handleTestSMTP(t, c)
	}()
	h, p, ok := strings.Cut(ln.Addr().String(), ":")
	if !ok {
		t.Fatal("bad listen addr", ln.Addr())
	}
	time.Sleep(20 * time.Millisecond)
	return h, p, func() {
		_ = ln.Close()
		wg.Wait()
	}
}

func handleTestSMTP(t *testing.T, conn net.Conn) {
	t.Helper()
	defer conn.Close()
	br := bufio.NewReader(conn)
	write := func(s string) { _, _ = conn.Write([]byte(s)) }
	write("220 test ESMTP\r\n")
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			return
		}
		up := strings.ToUpper(strings.TrimSpace(line))
		switch {
		case strings.HasPrefix(up, "EHLO"), strings.HasPrefix(up, "HELO"):
			// RFC 5321: continuation lines use "250-", last line must be "250 ".
			write("250-test Hello\r\n250 HELP\r\n")
		case strings.HasPrefix(up, "MAIL FROM"):
			write("250 OK\r\n")
		case strings.HasPrefix(up, "RCPT TO"):
			write("250 OK\r\n")
		case strings.HasPrefix(up, "DATA"):
			write("354 go ahead\r\n")
			for {
				l, err := br.ReadString('\n')
				if err != nil {
					return
				}
				if strings.TrimSpace(l) == "." {
					break
				}
			}
			write("250 OK queued\r\n")
		case strings.HasPrefix(up, "QUIT"):
			write("221 bye\r\n")
			return
		default:
			write(fmt.Sprintf("502 unimplemented: %q\r\n", strings.TrimSpace(line)))
		}
	}
}

func TestSMTP_Send_HappyPath(t *testing.T) {
	host, port, cleanup := runTestSMTPServer(t)
	defer cleanup()
	p := &SMTPProvider{
		Host: host,
		Port: port,
		From: "from@example.com",
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := p.Send(ctx, "to@example.com", "Hi", "<p>body</p>"); err != nil {
		t.Fatal(err)
	}
}
