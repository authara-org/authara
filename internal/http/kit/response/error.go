package response

type ErrorCode string

const (
	CodeUnauthorized ErrorCode = "unauthorized"
	CodeForbidden    ErrorCode = "forbidden"

	CodeInvalidRequest ErrorCode = "invalid_request"
	CodeNotFound       ErrorCode = "not_found"
	CodeRateLimited    ErrorCode = "rate_limited"

	CodeInternalError ErrorCode = "internal_error"
)

type Error struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
}

type ErrorResponse struct {
	Error Error `json:"error"`
}
