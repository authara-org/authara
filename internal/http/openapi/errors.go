package openapi

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/authara-org/authara/internal/http/kit/response"
	"github.com/getkin/kin-openapi/openapi3"
)

var (
	specOnce sync.Once
	spec     *openapi3.T
	specErr  error
)

func MustOperationErrors(operationID string) map[response.ErrorCode]response.ErrorSpec {
	out, err := operationErrors(operationID)
	if err != nil {
		panic(err)
	}
	return out
}

func ErrorCodeAllowed(extensions map[string]any, status int, code response.ErrorCode) bool {
	specs, err := errorSpecsFromExtensions("", extensions, nil)
	if err != nil {
		return false
	}
	spec, ok := specs[code]
	return ok && spec.Status == status
}

func operationErrors(operationID string) (map[response.ErrorCode]response.ErrorSpec, error) {
	specOnce.Do(func() {
		spec, specErr = GetSpec()
	})
	if specErr != nil {
		return nil, specErr
	}

	op := findOperation(operationID)
	if op == nil {
		return nil, fmt.Errorf("openapi operation %q not found", operationID)
	}

	return errorSpecsFromExtensions(operationID, op.Extensions, op.Responses)
}

func errorSpecsFromExtensions(operationID string, extensions map[string]any, responses *openapi3.Responses) (map[response.ErrorCode]response.ErrorSpec, error) {
	raw, ok := extensions["x-authara-error-codes"]
	if !ok {
		return map[response.ErrorCode]response.ErrorSpec{}, nil
	}
	statusCodes, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s has invalid x-authara-error-codes", operationErrorContext(operationID))
	}

	out := make(map[response.ErrorCode]response.ErrorSpec)
	for statusText, rawCodes := range statusCodes {
		status, err := strconv.Atoi(statusText)
		if err != nil {
			return nil, fmt.Errorf("%s has invalid error status %q", operationErrorContext(operationID), statusText)
		}
		if status < http.StatusBadRequest {
			return nil, fmt.Errorf("%s declares non-error status %d in x-authara-error-codes", operationErrorContext(operationID), status)
		}
		if responses != nil && responses.Status(status) == nil {
			return nil, fmt.Errorf("%s declares x-authara-error-codes status %d without a response", operationErrorContext(operationID), status)
		}
		codes, ok := rawCodes.([]any)
		if !ok {
			return nil, fmt.Errorf("%s has invalid error codes for status %d", operationErrorContext(operationID), status)
		}
		for _, rawCode := range codes {
			code, ok := rawCode.(string)
			if !ok || code == "" {
				return nil, fmt.Errorf("%s has invalid error code for status %d", operationErrorContext(operationID), status)
			}
			errorCode := response.ErrorCode(code)
			if previous, exists := out[errorCode]; exists {
				return nil, fmt.Errorf("%s declares error code %q for both status %d and %d", operationErrorContext(operationID), code, previous.Status, status)
			}
			out[errorCode] = response.ErrorSpec{Status: status, Code: errorCode}
		}
	}
	return out, nil
}

func findOperation(operationID string) *openapi3.Operation {
	for _, path := range spec.Paths.Keys() {
		item := spec.Paths.Value(path)
		if item == nil {
			continue
		}
		for _, op := range item.Operations() {
			if op != nil && strings.EqualFold(op.OperationID, operationID) {
				return op
			}
		}
	}
	return nil
}

func operationErrorContext(operationID string) string {
	if operationID == "" {
		return "openapi operation"
	}
	return fmt.Sprintf("openapi operation %q", operationID)
}
