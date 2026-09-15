package email

import (
	"bufio"
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/textproto"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestClassifyFailureUsesSMTPStatusClass(t *testing.T) {
	tests := []struct {
		name      string
		code      int
		wantClass FailureClass
	}{
		{name: "temporary mailbox failure", code: 450, wantClass: FailureTransient},
		{name: "service unavailable", code: 421, wantClass: FailureTransient},
		{name: "authentication rejected", code: 535, wantClass: FailurePermanent},
		{name: "recipient rejected", code: 550, wantClass: FailurePermanent},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := classifySMTPError("smtp_test_failure", &textproto.Error{Code: test.code, Msg: "test"})
			gotClass, gotReason := ClassifyFailure(err)
			if gotClass != test.wantClass || gotReason != "smtp_test_failure" {
				t.Fatalf("classification = (%q, %q), want (%q, %q)", gotClass, gotReason, test.wantClass, "smtp_test_failure")
			}
		})
	}
}

func TestSMTPSenderClassifiesRecipientResponses(t *testing.T) {
	tests := []struct {
		code      int
		wantClass FailureClass
	}{
		{code: 450, wantClass: FailureTransient},
		{code: 550, wantClass: FailurePermanent},
	}

	for _, test := range tests {
		t.Run(strconv.Itoa(test.code), func(t *testing.T) {
			host, port := startRecipientRejectingSMTPServer(t, test.code)
			sender := NewSMTPSender(host, port, "", "", "sender@example.test", false, time.Second)
			err := sender.Send(t.Context(), "recipient@example.test", Message{Subject: "test", Text: "test"})
			class, reason := ClassifyFailure(err)
			if class != test.wantClass || reason != "smtp_recipient_rejected" {
				t.Fatalf("classification = (%q, %q, %v), want (%q, %q)", class, reason, err, test.wantClass, "smtp_recipient_rejected")
			}
		})
	}
}

func TestSMTPSenderRejectsInvalidConfigurationPermanently(t *testing.T) {
	tests := []struct {
		name   string
		from   string
		to     string
		reason string
	}{
		{name: "invalid sender", from: "not an address", to: "user@example.com", reason: "invalid_sender"},
		{name: "invalid recipient", from: "sender@example.com", to: "not an address", reason: "invalid_recipient"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			sender := NewSMTPSender("smtp.example.com", 587, "", "", test.from, true, 0)
			err := sender.Send(t.Context(), test.to, Message{Subject: "test", Text: "test"})
			if err == nil {
				t.Fatal("Send returned nil")
			}
			class, reason := ClassifyFailure(err)
			if class != FailurePermanent || reason != test.reason {
				t.Fatalf("classification = (%q, %q), want (%q, %q): %v", class, reason, FailurePermanent, test.reason, err)
			}
			var deliveryErr *DeliveryError
			if !errors.As(err, &deliveryErr) {
				t.Fatalf("Send error %T is not a DeliveryError", err)
			}
		})
	}
}

func TestSMTPSenderRejectsIncompleteCredentialsPermanently(t *testing.T) {
	sender := NewSMTPSender("smtp.example.com", 587, "user", "", "sender@example.com", true, time.Second)
	err := sender.Send(t.Context(), "recipient@example.com", Message{Subject: "test", Text: "test"})
	class, reason := ClassifyFailure(err)
	if class != FailurePermanent || reason != "configuration_error" {
		t.Fatalf("classification = (%q, %q, %v)", class, reason, err)
	}
}

func TestSMTPTLSCertificateFailureIsPermanent(t *testing.T) {
	err := classifySMTPTLSError(fmt.Errorf("starttls: %w", x509.UnknownAuthorityError{}))
	class, reason := ClassifyFailure(err)
	if class != FailurePermanent || reason != "smtp_tls_failed" {
		t.Fatalf("classification = (%q, %q, %v)", class, reason, err)
	}
}

func TestSMTPSenderTreatsQuitFailureAfterAcceptanceAsSuccess(t *testing.T) {
	host, port := startSMTPServerClosingAfterData(t)
	sender := NewSMTPSender(host, port, "", "", "sender@example.test", false, time.Second)

	if err := sender.Send(t.Context(), "recipient@example.test", Message{Subject: "test", Text: "test"}); err != nil {
		t.Fatalf("Send returned an error after DATA was accepted: %v", err)
	}
}

func TestSMTPSenderCancellationClosesActiveConnection(t *testing.T) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	accepted := make(chan struct{})
	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			return
		}
		defer conn.Close()
		close(accepted)
		buffer := make([]byte, 1)
		_, _ = conn.Read(buffer)
	}()

	address := listener.Addr().(*net.TCPAddr)
	sender := NewSMTPSender(address.IP.String(), address.Port, "", "", "sender@example.test", false, 5*time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		result <- sender.Send(ctx, "recipient@example.test", Message{Subject: "test", Text: "test"})
	}()
	select {
	case <-accepted:
	case <-time.After(time.Second):
		t.Fatal("SMTP connection was not accepted")
	}
	cancel()

	select {
	case sendErr := <-result:
		class, _ := ClassifyFailure(sendErr)
		if class != FailureTransient {
			t.Fatalf("cancelled send classification = %q: %v", class, sendErr)
		}
	case <-time.After(time.Second):
		t.Fatal("cancelled SMTP send did not return promptly")
	}
	<-serverDone
}

func startRecipientRejectingSMTPServer(t *testing.T, code int) (string, int) {
	t.Helper()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })

	done := make(chan error, 1)
	go func() {
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			done <- acceptErr
			return
		}
		defer conn.Close()
		reader := bufio.NewReader(conn)
		writer := bufio.NewWriter(conn)
		writeResponse := func(response string) error {
			if _, writeErr := writer.WriteString(response + "\r\n"); writeErr != nil {
				return writeErr
			}
			return writer.Flush()
		}
		readCommand := func(prefix string) error {
			line, readErr := reader.ReadString('\n')
			if readErr != nil {
				return readErr
			}
			if !strings.HasPrefix(strings.ToUpper(line), prefix) {
				return fmt.Errorf("command = %q, want prefix %q", line, prefix)
			}
			return nil
		}

		if err := writeResponse("220 smtp.example.test ESMTP"); err != nil {
			done <- err
			return
		}
		if err := readCommand("EHLO"); err != nil {
			done <- err
			return
		}
		if err := writeResponse("250 smtp.example.test"); err != nil {
			done <- err
			return
		}
		if err := readCommand("MAIL FROM:"); err != nil {
			done <- err
			return
		}
		if err := writeResponse("250 sender accepted"); err != nil {
			done <- err
			return
		}
		if err := readCommand("RCPT TO:"); err != nil {
			done <- err
			return
		}
		done <- writeResponse(fmt.Sprintf("%d recipient rejected", code))
	}()

	address := listener.Addr().(*net.TCPAddr)
	t.Cleanup(func() {
		select {
		case serveErr := <-done:
			if serveErr != nil && !errors.Is(serveErr, net.ErrClosed) {
				t.Errorf("SMTP test server: %v", serveErr)
			}
		case <-time.After(time.Second):
			t.Error("SMTP test server did not exit")
		}
	})
	return address.IP.String(), address.Port
}

func startSMTPServerClosingAfterData(t *testing.T) (string, int) {
	t.Helper()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })

	done := make(chan error, 1)
	go func() {
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			done <- acceptErr
			return
		}
		defer conn.Close()
		reader := bufio.NewReader(conn)
		writer := bufio.NewWriter(conn)
		writeResponse := func(response string) error {
			if _, writeErr := writer.WriteString(response + "\r\n"); writeErr != nil {
				return writeErr
			}
			return writer.Flush()
		}
		readCommand := func(prefix string) error {
			line, readErr := reader.ReadString('\n')
			if readErr != nil {
				return readErr
			}
			if !strings.HasPrefix(strings.ToUpper(line), prefix) {
				return fmt.Errorf("command = %q, want prefix %q", line, prefix)
			}
			return nil
		}

		steps := []struct {
			command  string
			response string
		}{
			{command: "", response: "220 smtp.example.test ESMTP"},
			{command: "EHLO", response: "250 smtp.example.test"},
			{command: "MAIL FROM:", response: "250 sender accepted"},
			{command: "RCPT TO:", response: "250 recipient accepted"},
			{command: "DATA", response: "354 end with dot"},
		}
		for _, step := range steps {
			if step.command != "" {
				if err := readCommand(step.command); err != nil {
					done <- err
					return
				}
			}
			if err := writeResponse(step.response); err != nil {
				done <- err
				return
			}
		}
		if _, err := textproto.NewReader(reader).ReadDotBytes(); err != nil {
			done <- err
			return
		}
		done <- writeResponse("250 message accepted")
	}()

	address := listener.Addr().(*net.TCPAddr)
	t.Cleanup(func() {
		select {
		case serveErr := <-done:
			if serveErr != nil && !errors.Is(serveErr, net.ErrClosed) {
				t.Errorf("SMTP test server: %v", serveErr)
			}
		case <-time.After(time.Second):
			t.Error("SMTP test server did not exit")
		}
	})
	return address.IP.String(), address.Port
}
