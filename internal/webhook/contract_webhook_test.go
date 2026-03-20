package webhook

import (
	"encoding/json"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

type webhookContract struct {
	Version   int                    `yaml:"version"`
	Events    []webhookContractEvent `yaml:"events"`
	Signature webhookSignatureSpec   `yaml:"signature"`
	Delivery  webhookDeliverySpec    `yaml:"delivery"`
}

type webhookContractEvent struct {
	Type    string                 `yaml:"type"`
	Headers []string               `yaml:"headers"`
	Payload webhookContractPayload `yaml:"payload"`
}

type webhookContractPayload struct {
	ID        string                     `yaml:"id"`
	Type      string                     `yaml:"type"`
	CreatedAt string                     `yaml:"created_at"`
	Data      webhookContractPayloadData `yaml:"data"`
}

type webhookContractPayloadData struct {
	UserID string `yaml:"user_id"`
}

type webhookSignatureSpec struct {
	Header    string `yaml:"header"`
	Format    string `yaml:"format"`
	Algorithm string `yaml:"algorithm"`
}

type webhookDeliverySpec struct {
	Method      string               `yaml:"method"`
	ContentType string               `yaml:"content_type"`
	Semantics   string               `yaml:"semantics"`
	Retries     webhookRetryContract `yaml:"retries"`
}

type webhookRetryContract struct {
	On    []string `yaml:"on"`
	NotOn []string `yaml:"not_on"`
}

func TestWebhookContract_EventTypesMatchCode(t *testing.T) {
	contract := loadWebhookContract(t)

	contractTypes := make([]string, 0, len(contract.Events))
	for _, evt := range contract.Events {
		contractTypes = append(contractTypes, evt.Type)
	}

	codeTypes := []string{}
	for _, t := range SupportedEventTypes {
		codeTypes = append(codeTypes, string(t))
	}

	slices.Sort(contractTypes)
	slices.Sort(codeTypes)

	if !slices.Equal(contractTypes, codeTypes) {
		t.Fatalf("event types mismatch: contract=%v code=%v", contractTypes, codeTypes)
	}
}

func TestWebhookContract_SignatureFormatMatchesCode(t *testing.T) {
	contract := loadWebhookContract(t)

	if contract.Signature.Header != SignatureHeader {
		t.Fatalf("expected signature header %q, got %q", SignatureHeader, contract.Signature.Header)
	}
	if contract.Signature.Format != SignatureFormat {
		t.Fatalf("expected signature format %q, got %q", SignatureFormat, contract.Signature.Format)
	}
	if contract.Signature.Algorithm != SignatureAlgorithm {
		t.Fatalf("expected signature algorithm %q, got %q", SignatureAlgorithm, contract.Signature.Algorithm)
	}

	got := Sign("secret", []byte(`{"hello":"world"}`))
	if !strings.HasPrefix(got, SignaturePrefix) {
		t.Fatalf("expected signature to start with %q, got %q", SignaturePrefix, got)
	}
	if len(got) <= len(SignaturePrefix) {
		t.Fatalf("expected non-empty signature body, got %q", got)
	}
}

func TestWebhookContract_DeliverySettingsMatchCode(t *testing.T) {
	contract := loadWebhookContract(t)

	if contract.Delivery.Method != "POST" {
		t.Fatalf("expected delivery method POST, got %q", contract.Delivery.Method)
	}
	if contract.Delivery.ContentType != "application/json" {
		t.Fatalf("expected content type application/json, got %q", contract.Delivery.ContentType)
	}
}

func TestWebhookContract_HeadersMatchCode(t *testing.T) {
	contract := loadWebhookContract(t)

	expectedHeaders := []string{
		EventHeader,
		DeliveryHeader,
		SignatureHeader,
	}
	slices.Sort(expectedHeaders)

	for _, evt := range contract.Events {
		got := slices.Clone(evt.Headers)
		slices.Sort(got)

		if !slices.Equal(got, expectedHeaders) {
			t.Fatalf("headers mismatch for event %q: contract=%v code=%v", evt.Type, got, expectedHeaders)
		}
	}
}

func TestWebhookContract_RetrySemanticsMatchCode(t *testing.T) {
	contract := loadWebhookContract(t)

	if contract.Delivery.Semantics != DeliverySemantics {
		t.Fatalf("expected delivery semantics %q, got %q", DeliverySemantics, contract.Delivery.Semantics)
	}

	expectedRetryOn := slices.Clone(RetryOn)
	expectedRetryNotOn := slices.Clone(RetryNotOn)

	gotRetryOn := slices.Clone(contract.Delivery.Retries.On)
	gotRetryNotOn := slices.Clone(contract.Delivery.Retries.NotOn)

	slices.Sort(expectedRetryOn)
	slices.Sort(expectedRetryNotOn)
	slices.Sort(gotRetryOn)
	slices.Sort(gotRetryNotOn)

	if !slices.Equal(gotRetryOn, expectedRetryOn) {
		t.Fatalf("retry on mismatch: contract=%v code=%v", gotRetryOn, expectedRetryOn)
	}
	if !slices.Equal(gotRetryNotOn, expectedRetryNotOn) {
		t.Fatalf("retry not_on mismatch: contract=%v code=%v", gotRetryNotOn, expectedRetryNotOn)
	}
}

func TestWebhookContract_UserCreatedPayloadShape(t *testing.T) {
	contract := loadWebhookContract(t)
	spec := mustFindEventSpec(t, contract, string(EventUserCreated))

	evt := NewUserCreated(uuid.New(), time.Now())
	assertEnvelopeMatchesContract(t, evt, spec)
}

func TestWebhookContract_UserDeletedPayloadShape(t *testing.T) {
	contract := loadWebhookContract(t)
	spec := mustFindEventSpec(t, contract, string(EventUserDeleted))

	evt := NewUserDeleted(uuid.New(), time.Now())
	assertEnvelopeMatchesContract(t, evt, spec)
}

func loadWebhookContract(t *testing.T) webhookContract {
	t.Helper()

	data, err := os.ReadFile("../../contract/webhooks.yaml")
	if err != nil {
		t.Fatalf("read contract/webhooks.yaml: %v", err)
	}

	var contract webhookContract
	if err := yaml.Unmarshal(data, &contract); err != nil {
		t.Fatalf("unmarshal contract/webhooks.yaml: %v", err)
	}

	return contract
}

func mustFindEventSpec(t *testing.T, contract webhookContract, eventType string) webhookContractEvent {
	t.Helper()

	for _, evt := range contract.Events {
		if evt.Type == eventType {
			return evt
		}
	}

	t.Fatalf("event %q not found in contract/webhooks.yaml", eventType)
	return webhookContractEvent{}
}

func assertEnvelopeMatchesContract(t *testing.T, evt Envelope, spec webhookContractEvent) {
	t.Helper()

	raw, err := json.Marshal(evt)
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}

	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatalf("unmarshal marshaled event: %v", err)
	}

	// top-level fields
	assertFieldPresent(t, body, "id")
	assertFieldPresent(t, body, "type")
	assertFieldPresent(t, body, "created_at")
	assertFieldPresent(t, body, "data")

	if spec.Payload.ID != "string" {
		t.Fatalf("expected contract payload id type string, got %q", spec.Payload.ID)
	}
	if spec.Payload.Type != string(evt.Type) {
		t.Fatalf("expected contract payload type %q, got %q", evt.Type, spec.Payload.Type)
	}
	if spec.Payload.CreatedAt != "rfc3339" {
		t.Fatalf("expected contract payload created_at type rfc3339, got %q", spec.Payload.CreatedAt)
	}
	if spec.Payload.Data.UserID != "uuid" {
		t.Fatalf("expected contract payload data.user_id type uuid, got %q", spec.Payload.Data.UserID)
	}

	id, ok := body["id"].(string)
	if !ok || id == "" {
		t.Fatalf("expected non-empty string id, got %#v", body["id"])
	}

	eventType, ok := body["type"].(string)
	if !ok || eventType != string(evt.Type) {
		t.Fatalf("expected type %q, got %#v", evt.Type, body["type"])
	}

	createdAt, ok := body["created_at"].(string)
	if !ok || createdAt == "" {
		t.Fatalf("expected non-empty created_at string, got %#v", body["created_at"])
	}
	if _, err := time.Parse(time.RFC3339, createdAt); err != nil {
		t.Fatalf("expected RFC3339 created_at, got %q: %v", createdAt, err)
	}

	data, ok := body["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected data object, got %#v", body["data"])
	}
	assertFieldPresent(t, data, "user_id")

	userID, ok := data["user_id"].(string)
	if !ok || userID == "" {
		t.Fatalf("expected non-empty user_id string, got %#v", data["user_id"])
	}
	if _, err := uuid.Parse(userID); err != nil {
		t.Fatalf("expected valid uuid user_id, got %q: %v", userID, err)
	}
}

func assertFieldPresent(t *testing.T, obj map[string]any, field string) {
	t.Helper()

	if _, ok := obj[field]; !ok {
		t.Fatalf("expected field %q to be present, got object %#v", field, obj)
	}
}
