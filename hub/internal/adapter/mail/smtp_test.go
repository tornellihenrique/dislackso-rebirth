package mail

import (
	"bufio"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/ports"
)

// fakeSMTP accepts one plain-text SMTP session and returns the DATA it received.
func fakeSMTP(t *testing.T) (port int, received <-chan string) {
	t.Helper()
	var lc net.ListenConfig
	ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	out := make(chan string, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
		r := bufio.NewReader(conn)
		reply := func(s string) { _, _ = conn.Write([]byte(s + "\r\n")) }
		reply("220 fake ESMTP")
		var data strings.Builder
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				return
			}
			cmd := strings.ToUpper(strings.TrimSpace(line))
			switch {
			case strings.HasPrefix(cmd, "EHLO"), strings.HasPrefix(cmd, "HELO"):
				reply("250 fake")
			case strings.HasPrefix(cmd, "MAIL"), strings.HasPrefix(cmd, "RCPT"), strings.HasPrefix(cmd, "RSET"), strings.HasPrefix(cmd, "NOOP"):
				reply("250 ok")
			case cmd == "DATA":
				reply("354 go ahead")
				for {
					l, err := r.ReadString('\n')
					if err != nil {
						return
					}
					if l == ".\r\n" {
						break
					}
					data.WriteString(l)
				}
				reply("250 queued")
				out <- data.String()
			case cmd == "QUIT":
				reply("221 bye")
				return
			default:
				reply("502 not implemented")
			}
		}
	}()
	addr, ok := ln.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatal("not a TCP listener")
	}
	return addr.Port, out
}

func TestSMTPSendsMultipartMessage(t *testing.T) {
	port, received := fakeSMTP(t)
	settings := domain.SMTPSettings{
		Enabled: true, Host: "127.0.0.1", Port: port, Security: domain.SMTPNone,
		FromAddress: "hub@example.com", FromName: "Team Hub",
	}
	m := ports.Mail{To: "ana@example.com", Subject: "Confirm your email", Text: "plain body", HTML: "<p>html body</p>"}

	if err := (SMTP{}).SendWith(t.Context(), settings, "", m); err != nil {
		t.Fatalf("SendWith: %v", err)
	}
	select {
	case data := <-received:
		for _, want := range []string{
			`From: "Team Hub" <hub@example.com>`,
			"To: <ana@example.com>",
			"Subject: Confirm your email",
			"multipart/alternative",
			"plain body",
			"<p>html body</p>",
		} {
			if !strings.Contains(data, want) {
				t.Errorf("message lacks %q:\n%s", want, data)
			}
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the server received nothing")
	}
}

func TestSMTPReportsConnectionFailure(t *testing.T) {
	settings := domain.SMTPSettings{Enabled: true, Host: "127.0.0.1", Port: 1, Security: domain.SMTPNone, FromAddress: "hub@example.com"}
	if err := (SMTP{}).SendWith(t.Context(), settings, "", ports.Mail{To: "a@example.com", Subject: "x", Text: "x"}); err == nil {
		t.Fatal("expected an error for a closed port")
	}
}
