package webhook

import (
	"encoding/json"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/authara-org/authara/internal/domain"
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
	ID        string            `yaml:"id"`
	Type      string            `yaml:"type"`
	CreatedAt string            `yaml:"created_at"`
	Data      map[string]string `yaml:"data"`
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

func TestWebhookContract_OrganizationInvitationCreatedPayloadShape(t *testing.T) {
	contract := loadWebhookContract(t)
	spec := mustFindEventSpec(t, contract, string(EventOrganizationInvitationCreated))

	actorID := uuid.New()
	evt := NewOrganizationInvitationCreated(fakeInvitation(&actorID), time.Now())
	assertEnvelopeMatchesContract(t, evt, spec)
}

func TestWebhookContract_OrganizationInvitationAcceptedPayloadShape(t *testing.T) {
	contract := loadWebhookContract(t)
	spec := mustFindEventSpec(t, contract, string(EventOrganizationInvitationAccepted))

	acceptedBy := uuid.New()
	acceptedAt := time.Now().UTC()
	invitation := fakeInvitation(nil)
	invitation.AcceptedAt = &acceptedAt
	invitation.AcceptedByUserID = &acceptedBy

	evt := NewOrganizationInvitationAccepted(invitation, time.Now())
	assertEnvelopeMatchesContract(t, evt, spec)
}

func TestWebhookContract_OrganizationMembershipCreatedPayloadShape(t *testing.T) {
	contract := loadWebhookContract(t)
	spec := mustFindEventSpec(t, contract, string(EventOrganizationMembershipCreated))

	evt := NewOrganizationMembershipCreated(domain.OrganizationMembership{
		OrganizationID: uuid.New(),
		UserID:         uuid.New(),
		Role:           domain.OrganizationRoleMember,
	}, time.Now())
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

	for field, fieldType := range spec.Payload.Data {
		assertContractDataField(t, data, field, fieldType)
	}
}

func assertContractDataField(t *testing.T, data map[string]any, field string, fieldType string) {
	t.Helper()

	assertFieldPresent(t, data, field)
	value := data[field]

	switch fieldType {
	case "uuid":
		s, ok := value.(string)
		if !ok || s == "" {
			t.Fatalf("expected non-empty uuid string %q, got %#v", field, value)
		}
		if _, err := uuid.Parse(s); err != nil {
			t.Fatalf("expected valid uuid %q, got %q: %v", field, s, err)
		}
	case "uuid|null":
		if value == nil {
			return
		}
		s, ok := value.(string)
		if !ok || s == "" {
			t.Fatalf("expected uuid string or null %q, got %#v", field, value)
		}
		if _, err := uuid.Parse(s); err != nil {
			t.Fatalf("expected valid uuid %q, got %q: %v", field, s, err)
		}
	case "string":
		s, ok := value.(string)
		if !ok || s == "" {
			t.Fatalf("expected non-empty string %q, got %#v", field, value)
		}
	case "rfc3339":
		s, ok := value.(string)
		if !ok || s == "" {
			t.Fatalf("expected non-empty rfc3339 string %q, got %#v", field, value)
		}
		if _, err := time.Parse(time.RFC3339, s); err != nil {
			t.Fatalf("expected valid rfc3339 %q, got %q: %v", field, s, err)
		}
	default:
		t.Fatalf("unsupported contract data type %q for field %q", fieldType, field)
	}
}

func assertFieldPresent(t *testing.T, obj map[string]any, field string) {
	t.Helper()

	if _, ok := obj[field]; !ok {
		t.Fatalf("expected field %q to be present, got object %#v", field, obj)
	}
}

func fakeInvitation(invitedBy *uuid.UUID) domain.OrganizationInvitation {
	return domain.OrganizationInvitation{
		ID:              uuid.New(),
		OrganizationID:  uuid.New(),
		Email:           "teammate@example.com",
		Role:            domain.OrganizationRoleMember,
		InvitedByUserID: invitedBy,
		ExpiresAt:       time.Now().UTC().Add(24 * time.Hour),
	}
}
