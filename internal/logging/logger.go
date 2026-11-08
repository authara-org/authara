package logging

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

const MaxRecordBytes = 16 * 1024

type Level string

const (
	LevelDebug Level = "debug"
	LevelInfo  Level = "info"
	LevelWarn  Level = "warn"
	LevelError Level = "error"
)

type Event string

const (
	EventProcessLifecycle      Event = "process_lifecycle"
	EventConfigurationStatus   Event = "configuration_status"
	EventRequestCompleted      Event = "request_completed"
	EventAuthenticationOutcome Event = "authentication_outcome"
	EventWebhookDelivery       Event = "webhook_delivery"
	EventQueueOutcome          Event = "queue_outcome"
	EventWorkerOutcome         Event = "worker_outcome"
	EventMaintenanceOutcome    Event = "maintenance_outcome"
	EventInternalOperation     Event = "internal_operation"
	EventLogRecordDropped      Event = "log_record_dropped"
)

type Component string

const (
	ComponentProcess        Component = "process"
	ComponentHTTP           Component = "http"
	ComponentAuthentication Component = "authentication"
	ComponentWebhook        Component = "webhook"
	ComponentEmail          Component = "email"
	ComponentChallenge      Component = "challenge"
	ComponentQueue          Component = "queue"
	ComponentMaintenance    Component = "maintenance"
	ComponentConfiguration  Component = "configuration"
	ComponentDatabase       Component = "database"
	ComponentCache          Component = "cache"
	ComponentProvider       Component = "provider"
	ComponentOperator       Component = "operator"
	ComponentOther          Component = "other"
)

type Operation string

const (
	OperationStartup            Operation = "startup"
	OperationShutdown           Operation = "shutdown"
	OperationServeHTTP          Operation = "serve_http"
	OperationRequest            Operation = "request"
	OperationLogin              Operation = "login"
	OperationSignup             Operation = "signup"
	OperationReauthenticate     Operation = "reauthenticate"
	OperationAccountRecovery    Operation = "account_recovery"
	OperationAccountUpdate      Operation = "account_update"
	OperationProviderLink       Operation = "provider_link"
	OperationPasskey            Operation = "passkey"
	OperationRateLimit          Operation = "rate_limit"
	OperationContractValidation Operation = "contract_validation"
	OperationWebhookPublish     Operation = "webhook_publish"
	OperationWebhookDeliver     Operation = "webhook_deliver"
	OperationWebhookListen      Operation = "webhook_listen"
	OperationClaim              Operation = "claim"
	OperationDeliver            Operation = "deliver"
	OperationRetry              Operation = "retry"
	OperationRecover            Operation = "recover"
	OperationCleanup            Operation = "cleanup"
	OperationSnapshot           Operation = "snapshot"
	OperationLease              Operation = "lease"
	OperationConfiguration      Operation = "configuration"
	OperationTransportSecurity  Operation = "transport_security"
	OperationRevocation         Operation = "revocation"
	OperationResourceClose      Operation = "resource_close"
	OperationOther              Operation = "other"
)

type Outcome string

const (
	OutcomeStarted        Outcome = "started"
	OutcomeStopping       Outcome = "stopping"
	OutcomeStopped        Outcome = "stopped"
	OutcomeSucceeded      Outcome = "succeeded"
	OutcomeDenied         Outcome = "denied"
	OutcomeFailed         Outcome = "failed"
	OutcomeDegraded       Outcome = "degraded"
	OutcomeRetryScheduled Outcome = "retry_scheduled"
	OutcomeExhausted      Outcome = "exhausted"
	OutcomeRecovered      Outcome = "recovered"
	OutcomeSkipped        Outcome = "skipped"
	OutcomeIncomplete     Outcome = "incomplete"
	OutcomeClientError    Outcome = "client_error"
	OutcomeServerError    Outcome = "server_error"
	OutcomeOther          Outcome = "other"
)

type ErrorCode string

const (
	ErrorInvalidRequest        ErrorCode = "invalid_request"
	ErrorInvalidCredentials    ErrorCode = "invalid_credentials"
	ErrorAccessDenied          ErrorCode = "access_denied"
	ErrorRateLimited           ErrorCode = "rate_limited"
	ErrorConflict              ErrorCode = "conflict"
	ErrorNotFound              ErrorCode = "not_found"
	ErrorStorageUnavailable    ErrorCode = "storage_unavailable"
	ErrorDependencyUnavailable ErrorCode = "dependency_unavailable"
	ErrorProviderRejected      ErrorCode = "provider_rejected"
	ErrorDeliveryFailed        ErrorCode = "delivery_failed"
	ErrorTimeout               ErrorCode = "timeout"
	ErrorAttemptsExhausted     ErrorCode = "attempts_exhausted"
	ErrorDeadlineExceeded      ErrorCode = "deadline_exceeded"
	ErrorContractViolation     ErrorCode = "contract_violation"
	ErrorConfigurationInvalid  ErrorCode = "configuration_invalid"
	ErrorLeaseLost             ErrorCode = "lease_lost"
	ErrorResourceCleanup       ErrorCode = "resource_cleanup"
	ErrorInternalFailure       ErrorCode = "internal_failure"
	ErrorOther                 ErrorCode = "other"
)

type StatusClass string

const (
	StatusInformational StatusClass = "informational"
	StatusSuccess       StatusClass = "success"
	StatusRedirect      StatusClass = "redirect"
	StatusClientError   StatusClass = "client_error"
	StatusServerError   StatusClass = "server_error"
	StatusUnknown       StatusClass = "unknown"
)

type Queue string

const (
	QueueEmail   Queue = "email"
	QueueWebhook Queue = "webhook"
	QueueOther   Queue = "other"
)

type Worker string

const (
	WorkerEmail   Worker = "email"
	WorkerWebhook Worker = "webhook"
	WorkerOther   Worker = "other"
)

type Mode string

const (
	ModeImmediate Mode = "immediate"
	ModeExpiry    Mode = "expiry"
	ModeEnabled   Mode = "enabled"
	ModeDisabled  Mode = "disabled"
	ModeOther     Mode = "other"
)

type fieldKey string

const (
	keyRequestID         fieldKey = "request_id"
	keyComponent         fieldKey = "component"
	keyOperation         fieldKey = "operation"
	keyOutcome           fieldKey = "outcome"
	keyErrorCode         fieldKey = "error_code"
	keyMethod            fieldKey = "method"
	keyRoute             fieldKey = "route"
	keyStatusCode        fieldKey = "status_code"
	keyStatusClass       fieldKey = "status_class"
	keyDurationMS        fieldKey = "duration_ms"
	keyRequestSizeBytes  fieldKey = "request_size_bytes"
	keyResponseSizeBytes fieldKey = "response_size_bytes"
	keyAttempt           fieldKey = "attempt"
	keyRetryable         fieldKey = "retryable"
	keyQueue             fieldKey = "queue"
	keyWorker            fieldKey = "worker"
	keyWorkerCount       fieldKey = "worker_count"
	keyRetriedCount      fieldKey = "retried_count"
	keyFailedCount       fieldKey = "failed_count"
	keyGeneration        fieldKey = "generation"
	keyItemCount         fieldKey = "item_count"
	keyEnabled           fieldKey = "enabled"
	keyMode              fieldKey = "mode"
)

type Field struct {
	key   fieldKey
	value any
	omit  bool
}

func RequestID(value string) Field {
	return Field{key: keyRequestID, value: value, omit: !ValidRequestID(value)}
}

func ComponentField(value Component) Field {
	return Field{key: keyComponent, value: normalizeComponent(value)}
}
func OperationField(value Operation) Field {
	return Field{key: keyOperation, value: normalizeOperation(value)}
}
func OutcomeField(value Outcome) Field { return Field{key: keyOutcome, value: normalizeOutcome(value)} }
func ErrorCodeField(value ErrorCode) Field {
	return Field{key: keyErrorCode, value: normalizeErrorCode(value)}
}
func QueueField(value Queue) Field   { return Field{key: keyQueue, value: normalizeQueue(value)} }
func WorkerField(value Worker) Field { return Field{key: keyWorker, value: normalizeWorker(value)} }
func ModeField(value Mode) Field     { return Field{key: keyMode, value: normalizeMode(value)} }
func Retryable(value bool) Field     { return Field{key: keyRetryable, value: value} }
func Enabled(value bool) Field       { return Field{key: keyEnabled, value: value} }

func Method(value string) Field {
	value = strings.ToUpper(value)
	if _, ok := allowedMethods[value]; !ok {
		value = "OTHER"
	}
	return Field{key: keyMethod, value: value}
}

func Route(value string) Field {
	if !validRoute(value) {
		value = "unmatched"
	}
	return Field{key: keyRoute, value: value}
}

func StatusCode(value int) Field {
	if value < 100 || value > 599 {
		return Field{key: keyStatusCode, value: 0, omit: true}
	}
	return Field{key: keyStatusCode, value: value}
}

func StatusClassField(value int) Field {
	class := StatusUnknown
	switch {
	case value >= 100 && value < 200:
		class = StatusInformational
	case value >= 200 && value < 300:
		class = StatusSuccess
	case value >= 300 && value < 400:
		class = StatusRedirect
	case value >= 400 && value < 500:
		class = StatusClientError
	case value >= 500 && value < 600:
		class = StatusServerError
	}
	return Field{key: keyStatusClass, value: class}
}

func Duration(value time.Duration) Field {
	return boundedInteger(keyDurationMS, value.Milliseconds(), 0, 86_400_000)
}

func RequestSize(value int64) Field {
	return boundedInteger(keyRequestSizeBytes, value, 0, 1<<40)
}

func ResponseSize(value int64) Field {
	return boundedInteger(keyResponseSizeBytes, value, 0, 1<<40)
}

func Attempt(value int) Field     { return boundedInteger(keyAttempt, int64(value), 0, 10_000) }
func WorkerCount(value int) Field { return boundedInteger(keyWorkerCount, int64(value), 0, 10_000) }
func RetriedCount(value int64) Field {
	return boundedInteger(keyRetriedCount, value, 0, 1_000_000)
}
func FailedCount(value int64) Field { return boundedInteger(keyFailedCount, value, 0, 1_000_000) }
func Generation(value int64) Field  { return boundedInteger(keyGeneration, value, 0, 1<<53) }
func ItemCount(value int64) Field   { return boundedInteger(keyItemCount, value, 0, 1_000_000) }

func boundedInteger(key fieldKey, value, minValue, maxValue int64) Field {
	if value < minValue {
		value = minValue
	}
	if value > maxValue {
		value = maxValue
	}
	return Field{key: key, value: value}
}

type Config struct {
	Level       string
	Environment string
	Version     string
	InstanceID  string
	Writer      io.Writer
	Now         func() time.Time
}

type Logger struct {
	minLevel    Level
	environment string
	version     string
	instanceID  string
	writer      io.Writer
	now         func() time.Time
	mu          sync.Mutex
}

func New(cfg Config) (*Logger, error) {
	level, err := parseLevel(cfg.Level)
	if err != nil {
		return nil, err
	}
	environment, err := normalizeResource("environment", cfg.Environment, 32)
	if err != nil {
		return nil, err
	}
	version, err := normalizeResource("version", cfg.Version, 64)
	if err != nil {
		return nil, err
	}
	instanceID := cfg.InstanceID
	if instanceID == "" {
		instanceID = uuid.NewString()
	}
	if !ValidRequestID(instanceID) {
		return nil, fmt.Errorf("invalid logging instance ID")
	}
	writer := cfg.Writer
	if writer == nil {
		writer = os.Stdout
	}
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	return &Logger{minLevel: level, environment: environment, version: version, instanceID: instanceID, writer: writer, now: now}, nil
}

func Discard() *Logger {
	logger, _ := New(Config{Level: string(LevelError), Environment: "test", Version: "test", Writer: io.Discard})
	return logger
}

func (l *Logger) Debug(event Event, fields ...Field) {
	l.Log(context.Background(), LevelDebug, event, fields...)
}
func (l *Logger) Info(event Event, fields ...Field) {
	l.Log(context.Background(), LevelInfo, event, fields...)
}
func (l *Logger) Warn(event Event, fields ...Field) {
	l.Log(context.Background(), LevelWarn, event, fields...)
}
func (l *Logger) Error(event Event, fields ...Field) {
	l.Log(context.Background(), LevelError, event, fields...)
}

func (l *Logger) Log(ctx context.Context, level Level, event Event, fields ...Field) {
	if l == nil || !l.enabled(level) {
		return
	}
	if requestID := requestIDFromContext(ctx); requestID != "" && eventAllowsField(event, keyRequestID) && !containsField(fields, keyRequestID) {
		fields = append(fields, RequestID(requestID))
	}
	record, ok := l.buildRecord(level, event, fields)
	if !ok {
		record = l.baseRecord(LevelWarn, EventLogRecordDropped)
	}
	encoded, err := json.Marshal(record)
	if err != nil || len(encoded)+1 > MaxRecordBytes {
		encoded, _ = json.Marshal(l.baseRecord(LevelWarn, EventLogRecordDropped))
	}
	encoded = append(encoded, '\n')
	l.mu.Lock()
	_, _ = l.writer.Write(encoded)
	l.mu.Unlock()
}

func eventAllowsField(event Event, key fieldKey) bool {
	allowed, ok := eventFields[event]
	if !ok {
		return false
	}
	_, ok = allowed[key]
	return ok
}

func containsField(fields []Field, key fieldKey) bool {
	for _, field := range fields {
		if !field.omit && field.key == key {
			return true
		}
	}
	return false
}

func (l *Logger) buildRecord(level Level, event Event, fields []Field) (map[string]any, bool) {
	allowed, ok := eventFields[event]
	if !ok || !validLevel(level) {
		return nil, false
	}
	record := l.baseRecord(level, event)
	for _, field := range fields {
		if field.omit {
			continue
		}
		if _, ok := allowed[field.key]; !ok {
			return nil, false
		}
		if _, duplicate := record[string(field.key)]; duplicate {
			return nil, false
		}
		record[string(field.key)] = field.value
	}
	return record, true
}

func (l *Logger) baseRecord(level Level, event Event) map[string]any {
	return map[string]any{
		"timestamp":   l.now().UTC().Format(time.RFC3339Nano),
		"level":       level,
		"service":     "authara",
		"environment": l.environment,
		"version":     l.version,
		"instance_id": l.instanceID,
		"event":       event,
	}
}

func (l *Logger) enabled(level Level) bool {
	return levelRank(level) >= levelRank(l.minLevel)
}

var resourcePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`)
var routeSegmentPattern = regexp.MustCompile(`^[A-Za-z0-9._~-]+$`)
var routeParameterPattern = regexp.MustCompile(`^\{[A-Za-z][A-Za-z0-9_]*\}$`)

func normalizeResource(name, value string, maxLength int) (string, error) {
	value = strings.TrimSpace(value)
	if len(value) == 0 || len(value) > maxLength || !resourcePattern.MatchString(value) {
		return "", fmt.Errorf("invalid logging %s", name)
	}
	return value, nil
}

func ValidRequestID(value string) bool {
	id, err := uuid.Parse(value)
	if err != nil || id.String() != value {
		return false
	}
	return id.Version() == 4 || id.Version() == 7
}

func validRoute(value string) bool {
	if value == "" || len(value) > 256 || !strings.HasPrefix(value, "/") || strings.ContainsAny(value, "?#%\\\t\r\n ") {
		return false
	}
	if value == "/" {
		return true
	}
	for _, segment := range strings.Split(strings.TrimPrefix(value, "/"), "/") {
		if segment == "*" || routeSegmentPattern.MatchString(segment) || routeParameterPattern.MatchString(segment) {
			continue
		}
		return false
	}
	return true
}

var allowedMethods = map[string]struct{}{
	"CONNECT": {}, "DELETE": {}, "GET": {}, "HEAD": {}, "OPTIONS": {}, "PATCH": {}, "POST": {}, "PUT": {}, "TRACE": {}, "OTHER": {},
}

var eventFields = map[Event]map[fieldKey]struct{}{
	EventProcessLifecycle:      fields(keyComponent, keyOperation, keyOutcome, keyErrorCode, keyDurationMS, keyWorkerCount),
	EventConfigurationStatus:   fields(keyComponent, keyOperation, keyOutcome, keyErrorCode, keyEnabled, keyMode),
	EventRequestCompleted:      fields(keyRequestID, keyMethod, keyRoute, keyStatusCode, keyStatusClass, keyDurationMS, keyRequestSizeBytes, keyResponseSizeBytes, keyOutcome, keyErrorCode),
	EventAuthenticationOutcome: fields(keyRequestID, keyOperation, keyOutcome, keyErrorCode, keyMethod),
	EventWebhookDelivery:       fields(keyRequestID, keyOperation, keyOutcome, keyErrorCode, keyAttempt, keyDurationMS, keyRetryable),
	EventQueueOutcome:          fields(keyQueue, keyOperation, keyOutcome, keyErrorCode, keyAttempt, keyDurationMS, keyRetriedCount, keyFailedCount),
	EventWorkerOutcome:         fields(keyWorker, keyOperation, keyOutcome, keyErrorCode, keyAttempt, keyDurationMS, keyWorkerCount, keyRetriedCount, keyFailedCount),
	EventMaintenanceOutcome:    fields(keyOperation, keyOutcome, keyErrorCode, keyDurationMS, keyGeneration, keyItemCount),
	EventInternalOperation:     fields(keyRequestID, keyComponent, keyOperation, keyOutcome, keyErrorCode, keyItemCount),
	EventLogRecordDropped:      fields(),
}

func fields(keys ...fieldKey) map[fieldKey]struct{} {
	result := make(map[fieldKey]struct{}, len(keys))
	for _, key := range keys {
		result[key] = struct{}{}
	}
	return result
}

func parseLevel(value string) (Level, error) {
	switch strings.ToLower(value) {
	case string(LevelDebug):
		return LevelDebug, nil
	case string(LevelInfo):
		return LevelInfo, nil
	case string(LevelWarn), "warning":
		return LevelWarn, nil
	case string(LevelError):
		return LevelError, nil
	default:
		return LevelInfo, fmt.Errorf("invalid log level")
	}
}

func levelRank(value Level) int {
	switch value {
	case LevelDebug:
		return 0
	case LevelInfo:
		return 1
	case LevelWarn:
		return 2
	case LevelError:
		return 3
	default:
		return 4
	}
}

func validLevel(value Level) bool {
	return value == LevelDebug || value == LevelInfo || value == LevelWarn || value == LevelError
}

func normalizeComponent(value Component) Component {
	switch value {
	case ComponentProcess, ComponentHTTP, ComponentAuthentication, ComponentWebhook, ComponentEmail, ComponentChallenge, ComponentQueue, ComponentMaintenance, ComponentConfiguration, ComponentDatabase, ComponentCache, ComponentProvider, ComponentOperator, ComponentOther:
		return value
	default:
		return ComponentOther
	}
}

func normalizeOperation(value Operation) Operation {
	switch value {
	case OperationStartup, OperationShutdown, OperationServeHTTP, OperationRequest, OperationLogin, OperationSignup, OperationReauthenticate, OperationAccountRecovery, OperationAccountUpdate, OperationProviderLink, OperationPasskey, OperationRateLimit, OperationContractValidation, OperationWebhookPublish, OperationWebhookDeliver, OperationWebhookListen, OperationClaim, OperationDeliver, OperationRetry, OperationRecover, OperationCleanup, OperationSnapshot, OperationLease, OperationConfiguration, OperationTransportSecurity, OperationRevocation, OperationResourceClose, OperationOther:
		return value
	default:
		return OperationOther
	}
}

func normalizeOutcome(value Outcome) Outcome {
	switch value {
	case OutcomeStarted, OutcomeStopping, OutcomeStopped, OutcomeSucceeded, OutcomeDenied, OutcomeFailed, OutcomeDegraded, OutcomeRetryScheduled, OutcomeExhausted, OutcomeRecovered, OutcomeSkipped, OutcomeIncomplete, OutcomeClientError, OutcomeServerError, OutcomeOther:
		return value
	default:
		return OutcomeOther
	}
}

func normalizeErrorCode(value ErrorCode) ErrorCode {
	switch value {
	case ErrorInvalidRequest, ErrorInvalidCredentials, ErrorAccessDenied, ErrorRateLimited, ErrorConflict, ErrorNotFound, ErrorStorageUnavailable, ErrorDependencyUnavailable, ErrorProviderRejected, ErrorDeliveryFailed, ErrorTimeout, ErrorAttemptsExhausted, ErrorDeadlineExceeded, ErrorContractViolation, ErrorConfigurationInvalid, ErrorLeaseLost, ErrorResourceCleanup, ErrorInternalFailure, ErrorOther:
		return value
	default:
		return ErrorOther
	}
}

func normalizeQueue(value Queue) Queue {
	if value == QueueEmail || value == QueueWebhook || value == QueueOther {
		return value
	}
	return QueueOther
}

func normalizeWorker(value Worker) Worker {
	if value == WorkerEmail || value == WorkerWebhook || value == WorkerOther {
		return value
	}
	return WorkerOther
}

func normalizeMode(value Mode) Mode {
	if value == ModeImmediate || value == ModeExpiry || value == ModeEnabled || value == ModeDisabled || value == ModeOther {
		return value
	}
	return ModeOther
}
