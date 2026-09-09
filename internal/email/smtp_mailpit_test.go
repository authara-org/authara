package email

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/authara-org/authara/internal/domain"
	"github.com/google/uuid"
)

func TestMailpitReceivesRenderedTemplateOverride(t *testing.T) {
	mailpitURL := strings.TrimRight(os.Getenv("AUTHARA_TEST_MAILPIT_HTTP_URL"), "/")
	if mailpitURL == "" {
		t.Skip("AUTHARA_TEST_MAILPIT_HTTP_URL is not configured")
	}
	smtpHost := os.Getenv("AUTHARA_TEST_MAILPIT_SMTP_HOST")
	if smtpHost == "" {
		t.Fatal("AUTHARA_TEST_MAILPIT_SMTP_HOST must be set with AUTHARA_TEST_MAILPIT_HTTP_URL")
	}
	smtpPort := 1025
	if rawPort := os.Getenv("AUTHARA_TEST_MAILPIT_SMTP_PORT"); rawPort != "" {
		parsedPort, err := strconv.Atoi(rawPort)
		if err != nil {
			t.Fatalf("parse AUTHARA_TEST_MAILPIT_SMTP_PORT: %v", err)
		}
		smtpPort = parsedPort
	}

	httpClient := &http.Client{Timeout: 2 * time.Second}
	waitForMailpit(t, httpClient, mailpitURL)

	marker := strings.ReplaceAll(uuid.NewString(), "-", "")
	recipient := fmt.Sprintf("template-%s@example.test", marker)
	code := marker[:8]
	key := domain.EmailTemplateSignupCode
	templateStore := newFakeTemplateOverrideStore()
	templateStore.overrides[key] = domain.EmailTemplateOverride{
		Template:        key,
		SubjectTemplate: "Mailpit template " + marker,
		TextTemplate:    "Text verification code: {{code}} (" + marker + ")",
		HTMLTemplate:    "<p>HTML verification code: <strong>{{code}}</strong> (" + marker + ")</p>",
		Revision:        1,
	}
	message, err := NewTemplateService(templateStore).Render(context.Background(), key, TemplateData{
		TemplateVariableCode: code,
	})
	if err != nil {
		t.Fatalf("render template override: %v", err)
	}

	sender := NewSMTPSender(
		smtpHost,
		smtpPort,
		"",
		"",
		"Authara Test <no-reply@example.test>",
		false,
		5*time.Second,
	)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := sender.Send(ctx, recipient, message); err != nil {
		t.Fatalf("send rendered template to Mailpit: %v", err)
	}

	messageID := waitForMailpitMessage(t, httpClient, mailpitURL, recipient)
	t.Cleanup(func() {
		deleteMailpitMessage(t, httpClient, mailpitURL, messageID)
	})

	delivered := getMailpitMessage(t, httpClient, mailpitURL, messageID)
	if delivered.Subject != message.Subject {
		t.Fatalf("delivered subject = %q, want %q", delivered.Subject, message.Subject)
	}
	if !strings.Contains(delivered.Text, code) || !strings.Contains(delivered.Text, marker) {
		t.Fatalf("delivered text does not contain rendered values: %q", delivered.Text)
	}
	if !strings.Contains(delivered.HTML, "<strong>"+code+"</strong>") || !strings.Contains(delivered.HTML, marker) {
		t.Fatalf("delivered HTML does not contain rendered values: %q", delivered.HTML)
	}
}

type mailpitSearchResponse struct {
	Messages []struct {
		ID string `json:"ID"`
	} `json:"messages"`
}

type mailpitMessage struct {
	Subject string `json:"Subject"`
	Text    string `json:"Text"`
	HTML    string `json:"HTML"`
}

func waitForMailpit(t *testing.T, client *http.Client, baseURL string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		response, err := client.Get(baseURL + "/api/v1/info")
		if err == nil {
			_ = response.Body.Close()
			if response.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("Mailpit did not become ready at %s", baseURL)
}

func waitForMailpitMessage(t *testing.T, client *http.Client, baseURL, recipient string) string {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	searchURL := baseURL + "/api/v1/search?query=" + url.QueryEscape("to:"+recipient)
	for time.Now().Before(deadline) {
		response, err := client.Get(searchURL)
		if err == nil {
			var result mailpitSearchResponse
			decodeErr := json.NewDecoder(response.Body).Decode(&result)
			_ = response.Body.Close()
			if response.StatusCode == http.StatusOK && decodeErr == nil && len(result.Messages) > 0 {
				return result.Messages[0].ID
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("Mailpit did not receive message for %s", recipient)
	return ""
}

func getMailpitMessage(t *testing.T, client *http.Client, baseURL, messageID string) mailpitMessage {
	t.Helper()
	response, err := client.Get(baseURL + "/api/v1/message/" + url.PathEscape(messageID))
	if err != nil {
		t.Fatalf("get Mailpit message: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("get Mailpit message status = %d", response.StatusCode)
	}
	var message mailpitMessage
	if err := json.NewDecoder(response.Body).Decode(&message); err != nil {
		t.Fatalf("decode Mailpit message: %v", err)
	}
	return message
}

func deleteMailpitMessage(t *testing.T, client *http.Client, baseURL, messageID string) {
	t.Helper()
	body, err := json.Marshal(map[string][]string{"IDs": {messageID}})
	if err != nil {
		t.Errorf("encode Mailpit cleanup request: %v", err)
		return
	}
	request, err := http.NewRequest(http.MethodDelete, baseURL+"/api/v1/messages", bytes.NewReader(body))
	if err != nil {
		t.Errorf("create Mailpit cleanup request: %v", err)
		return
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := client.Do(request)
	if err != nil {
		t.Errorf("delete Mailpit test message: %v", err)
		return
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Errorf("delete Mailpit test message status = %d", response.StatusCode)
	}
}
