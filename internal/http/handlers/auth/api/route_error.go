package api

import "github.com/authara-org/authara/internal/http/kit/response"

func mustRouteError(
	errors map[response.ErrorCode]response.ErrorSpec,
	code response.ErrorCode,
) response.ErrorSpec {
	spec, ok := errors[code]
	if !ok {
		panic("undeclared route error: " + string(code))
	}
	return spec
}
