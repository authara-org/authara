package response

import (
	"encoding/json"
	"net/http"
)

type ErrorSpec struct {
	Status int
	Code   ErrorCode
}

func JSON(w http.ResponseWriter, status int, v any) {
	body, err := json.Marshal(v)
	if err != nil {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":{"code":"internal_error","message":"JSON encoding error."}}`))
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(append(body, '\n'))
}

func ErrorJSON(w http.ResponseWriter, status int, code ErrorCode, message string) {
	JSON(w, status, ErrorResponse{
		Error: Error{
			Code:    code,
			Message: message,
		},
	})
}

func WriteError(w http.ResponseWriter, spec ErrorSpec, message string) {
	ErrorJSON(w, spec.Status, spec.Code, message)
}
