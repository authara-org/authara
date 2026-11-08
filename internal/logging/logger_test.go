package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

const testInstanceID = "00000000-0000-4000-8000-000000000001"

func newBufferLogger(t *testing.T, buffer *bytes.Buffer) *Logger {
	t.Helper()
	logger, err := New(Config{
		Level: "debug", Environment: "staging", Version: "v1.2.3",
		InstanceID: testInstanceID, Writer: buffer,
		Now: func() time.Time { return time.Date(2026, 11, 8, 12, 0, 0, 123, time.UTC) },
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return logger
}

func decodeRecords(t *testing.T, output string) []map[string]any {
	t.Helper()
	lines := strings.Split(strings.TrimSpace(output), "\n")
	records := make([]map[string]any, len(lines))
	for i, line := range lines {
		if len(line)+1 > MaxRecordBytes {
			t.Fatalf("record %d is %d bytes", i, len(line)+1)
		}
		if err := json.Unmarshal([]byte(line), &records[i]); err != nil {
			t.Fatalf("record %d is not JSON: %v: %q", i, err, line)
		}
	}
	return records
}

func TestLoggerEmitsTypedPrivacySafeJSON(t *testing.T) {
	var output bytes.Buffer
	logger := newBufferLogger(t, &output)
	requestID := uuid.NewString()
	ctx := WithRequestID(context.Background(), requestID)

	logger.Log(ctx, LevelInfo, EventRequestCompleted,
		RequestID(requestID), Method("post"), Route("/auth/users/{userID}"),
		StatusCode(201), StatusClassField(201), Duration(5*time.Millisecond),
		RequestSize(42), ResponseSize(17), OutcomeField(OutcomeSucceeded),
	)

	record := decodeRecords(t, output.String())[0]
	want := map[string]any{
		"timestamp": "2026-11-08T12:00:00.000000123Z", "level": "info",
		"service": "authara", "environment": "staging", "version": "v1.2.3",
		"instance_id": testInstanceID, "event": "request_completed", "request_id": requestID,
		"method": "POST", "route": "/auth/users/{userID}", "status_code": float64(201),
		"status_class": "success", "duration_ms": float64(5),
		"request_size_bytes": float64(42), "response_size_bytes": float64(17), "outcome": "succeeded",
	}
	for key, value := range want {
		if record[key] != value {
			t.Errorf("%s = %#v, want %#v", key, record[key], value)
		}
	}
	for _, forbidden := range []string{"message", "error", "email", "user_id", "ip", "user_agent", "query"} {
		if _, ok := record[forbidden]; ok {
			t.Errorf("forbidden field %q present", forbidden)
		}
	}
}

func TestLoggerFailsClosedForUnknownEventFieldAndOversizedRecord(t *testing.T) {
	var output bytes.Buffer
	logger := newBufferLogger(t, &output)

	logger.Info(Event("invented_event"))
	logger.Info(EventProcessLifecycle, Field{key: fieldKey("secret"), value: "customer@example.com"})
	logger.Info(EventRequestCompleted, Field{key: keyRoute, value: strings.Repeat("x", MaxRecordBytes)})

	records := decodeRecords(t, output.String())
	if len(records) != 3 {
		t.Fatalf("records = %d, want 3", len(records))
	}
	for i, record := range records {
		if record["event"] != string(EventLogRecordDropped) {
			t.Errorf("record %d event = %#v", i, record["event"])
		}
		if len(record) != 7 {
			t.Errorf("record %d fields = %v", i, record)
		}
	}
	if strings.Contains(output.String(), "customer@example.com") {
		t.Fatal("dropped field value reached output")
	}
}

func TestRequestIDValidationAndRouteNormalization(t *testing.T) {
	v7, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("NewV7: %v", err)
	}
	for _, id := range []string{uuid.NewString(), v7.String()} {
		if !ValidRequestID(id) {
			t.Errorf("valid request ID rejected: %q", id)
		}
	}
	for _, id := range []string{"", "request-123", strings.ToUpper(uuid.NewString()), "00000000-0000-1000-8000-000000000001"} {
		if ValidRequestID(id) {
			t.Errorf("invalid request ID accepted: %q", id)
		}
	}
	if got := Route("/users/alice@example.com?token=secret").value; got != "unmatched" {
		t.Fatalf("unsafe route = %q", got)
	}
}

func TestLoggerWritesConcurrentRecordsAtomically(t *testing.T) {
	var output bytes.Buffer
	logger := newBufferLogger(t, &output)
	const count = 100
	var group sync.WaitGroup
	for range count {
		group.Add(1)
		go func() {
			defer group.Done()
			logger.Info(EventProcessLifecycle,
				ComponentField(ComponentProcess), OperationField(OperationStartup), OutcomeField(OutcomeStarted))
		}()
	}
	group.Wait()
	if records := decodeRecords(t, output.String()); len(records) != count {
		t.Fatalf("records = %d, want %d", len(records), count)
	}
}

func TestCorrelatedContextDoesNotInvalidateAnyEvent(t *testing.T) {
	var output bytes.Buffer
	logger := newBufferLogger(t, &output)
	ctx := WithRequestID(context.Background(), uuid.NewString())
	events := make([]Event, 0, len(eventFields))
	for event := range eventFields {
		events = append(events, event)
		logger.Log(ctx, LevelError, event)
	}
	for i, record := range decodeRecords(t, output.String()) {
		if record["event"] == string(EventLogRecordDropped) && events[i] != EventLogRecordDropped {
			t.Fatalf("record %d was dropped: %#v", i, record)
		}
	}
}

func TestStandardLoggerDiscardsLibraryMessage(t *testing.T) {
	var output bytes.Buffer
	logger := newBufferLogger(t, &output)
	logger.StandardLogger(ComponentHTTP, OperationServeHTTP).Print("panic for customer@example.com token=secret")
	record := decodeRecords(t, output.String())[0]
	if record["event"] != "internal_operation" || record["component"] != "http" {
		t.Fatalf("standard-library record = %#v", record)
	}
	if strings.Contains(output.String(), "customer@example.com") || strings.Contains(output.String(), "secret") {
		t.Fatal("standard-library message reached output")
	}
}
