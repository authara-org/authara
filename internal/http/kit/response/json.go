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
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(v); err != nil {
		http.Error(w, "json encoding error", http.StatusInternalServerError)
	}
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
