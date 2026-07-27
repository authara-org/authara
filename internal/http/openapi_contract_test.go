package http

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

var (
	operationIDPattern = regexp.MustCompile(`^[a-z][A-Za-z0-9]*$`)
	schemaNamePattern  = regexp.MustCompile(`^[A-Z][A-Za-z0-9]*$`)
)

func TestOpenAPIContract(t *testing.T) {
	loader := openapi3.NewLoader()
	document, err := loader.LoadFromFile("../../contract/openapi.yaml")
	if err != nil {
		t.Fatalf("load OpenAPI contract: %v", err)
	}
	if document.OpenAPI != "3.0.3" {
		t.Fatalf("expected OpenAPI 3.0.3, got %q", document.OpenAPI)
	}
	if err := document.Validate(context.Background()); err != nil {
		t.Fatalf("validate OpenAPI contract: %v", err)
	}

	topLevelTags := make(map[string]bool, len(document.Tags))
	for _, tag := range document.Tags {
		topLevelTags[tag.Name] = true
	}

	for name := range document.Components.Schemas {
		if !schemaNamePattern.MatchString(name) {
			t.Errorf("schema name %q must be stable PascalCase", name)
		}
	}

	operationIDs := make(map[string]string)
	for path, item := range document.Paths.Map() {
		for method, operation := range item.Operations() {
			endpoint := strings.ToUpper(method) + " " + path
			t.Run(endpoint, func(t *testing.T) {
				validateOpenAPIOperation(t, endpoint, operation, operationIDs, topLevelTags)
			})
		}
	}
}

func validateOpenAPIOperation(
	t *testing.T,
	endpoint string,
	operation *openapi3.Operation,
	operationIDs map[string]string,
	topLevelTags map[string]bool,
) {
	t.Helper()
	if operation.OperationID == "" {
		t.Fatal("operationId is required")
	}
	if !operationIDPattern.MatchString(operation.OperationID) {
		t.Fatalf("operationId %q must be lower camel case", operation.OperationID)
	}
	if previous, exists := operationIDs[operation.OperationID]; exists {
		t.Fatalf("operationId %q is also used by %s", operation.OperationID, previous)
	}
	operationIDs[operation.OperationID] = endpoint

	if len(operation.Tags) != 1 {
		t.Fatalf("expected exactly one tag, got %d", len(operation.Tags))
	}
	if !topLevelTags[operation.Tags[0]] {
		t.Fatalf("tag %q is not declared in top-level tags", operation.Tags[0])
	}
	if strings.TrimSpace(operation.Summary) == "" {
		t.Fatal("summary is required")
	}

	access, ok := operation.Extensions["x-authara-access"].(string)
	if !ok {
		t.Fatal("x-authara-access is required")
	}
	switch access {
	case "public", "user", "internal":
	default:
		t.Fatalf("invalid x-authara-access %q", access)
	}
	if access == "public" || access == "internal" || sdkFacingOperation(t, operation) {
		assertUsefulDescription(t, operation)
	}
	if sdkFacingOperation(t, operation) {
		assertSDKExamples(t, operation)
	}

	responses := operation.Responses.Map()
	if _, exists := responses["default"]; exists {
		t.Fatal("default responses are forbidden; every status must be explicit")
	}

	successes := 0
	actualErrors := make(map[string]bool)
	for status := range responses {
		code, err := strconv.Atoi(status)
		if err != nil {
			t.Fatalf("response status %q is not explicit", status)
		}
		if code >= 200 && code < 300 {
			successes++
		} else if code >= 400 && code < 600 {
			actualErrors[status] = true
			if responses[status].Ref != "#/components/responses/Error" {
				t.Fatalf("error response %s must reference #/components/responses/Error", status)
			}
		}
	}
	if successes != 1 {
		t.Fatalf("expected exactly one success response, got %d", successes)
	}

	declaredErrors := extensionErrorStatuses(t, operation)
	assertSameStatusSet(t, declaredErrors, actualErrors)
	if !containsString(extensionErrorCodes(t, operation, "500"), "internal_error") {
		t.Fatal("every operation must declare 500 internal_error for infrastructure failures")
	}

	if _, conditional := operation.Extensions["x-authara-availability"]; conditional {
		codes := extensionErrorCodes(t, operation, "404")
		if !containsString(codes, "not_found") {
			t.Fatal("conditionally available operations must declare 404 not_found")
		}
	}
}

func sdkFacingOperation(t *testing.T, operation *openapi3.Operation) bool {
	t.Helper()
	raw, exists := operation.Extensions["x-authara-sdk"]
	if !exists {
		return false
	}
	facing, ok := raw.(bool)
	if !ok {
		t.Fatalf("x-authara-sdk has type %T", raw)
	}
	return facing
}

func assertUsefulDescription(t *testing.T, operation *openapi3.Operation) {
	t.Helper()
	description := strings.TrimSpace(operation.Description)
	if description == "" {
		t.Fatal("description is required")
	}
	if description == strings.TrimSpace(operation.Summary) {
		t.Fatal("description must add detail beyond summary")
	}
}

func assertSDKExamples(t *testing.T, operation *openapi3.Operation) {
	t.Helper()
	if operation.RequestBody != nil && operation.RequestBody.Value != nil {
		for mediaType, content := range operation.RequestBody.Value.Content {
			if mediaType == "application/json" && !hasMediaExample(content) {
				t.Fatal("SDK-facing JSON request body must define an example")
			}
		}
	}

	for status, response := range operation.Responses.Map() {
		code, err := strconv.Atoi(status)
		if err != nil || code < 200 || code >= 300 || response.Value == nil {
			continue
		}
		for mediaType, content := range response.Value.Content {
			if mediaType == "application/json" && !hasMediaExample(content) {
				t.Fatal("SDK-facing JSON success response must define an example")
			}
		}
	}
}

func hasMediaExample(content *openapi3.MediaType) bool {
	return content != nil && (content.Example != nil || len(content.Examples) > 0)
}

func extensionErrorStatuses(t *testing.T, operation *openapi3.Operation) map[string]bool {
	t.Helper()
	raw, exists := operation.Extensions["x-authara-error-codes"]
	if !exists {
		return map[string]bool{}
	}
	errorsByStatus, ok := raw.(map[string]any)
	if !ok {
		t.Fatalf("x-authara-error-codes has type %T", raw)
	}
	statuses := make(map[string]bool, len(errorsByStatus))
	for status := range errorsByStatus {
		statuses[status] = true
	}
	return statuses
}

func extensionErrorCodes(t *testing.T, operation *openapi3.Operation, status string) []string {
	t.Helper()
	raw, exists := operation.Extensions["x-authara-error-codes"]
	if !exists {
		return nil
	}
	errorsByStatus, ok := raw.(map[string]any)
	if !ok {
		t.Fatalf("x-authara-error-codes has type %T", raw)
	}
	rawCodes, exists := errorsByStatus[status]
	if !exists {
		return nil
	}
	values, ok := rawCodes.([]any)
	if !ok {
		t.Fatalf("x-authara-error-codes[%s] has type %T", status, rawCodes)
	}
	codes := make([]string, 0, len(values))
	for _, value := range values {
		code, ok := value.(string)
		if !ok {
			t.Fatalf("x-authara-error-codes[%s] contains %T", status, value)
		}
		codes = append(codes, code)
	}
	return codes
}

func assertSameStatusSet(t *testing.T, want, got map[string]bool) {
	t.Helper()
	for status := range want {
		if !got[status] {
			t.Errorf("x-authara-error-codes declares %s but responses does not", status)
		}
	}
	for status := range got {
		if !want[status] {
			t.Errorf("responses declares %s but x-authara-error-codes does not", status)
		}
	}
}

func containsString(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func operationKey(method, path string) string {
	return fmt.Sprintf("%s %s", strings.ToUpper(method), path)
}
