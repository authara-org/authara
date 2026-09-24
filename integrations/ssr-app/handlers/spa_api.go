package handlers

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

const spaAPIMaxBodyBytes = 1 << 20

// RequireSPACSRF applies the same double-submit cookie check as Authara. The
// SPA gets this token from /auth/api/v1/csrf, so its backend needs no copy of
// Authara's secrets.
func RequireSPACSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("authara_csrf")
		header := r.Header.Get("X-CSRF-Token")
		if err != nil || cookie.Value == "" || header == "" || subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(header)) != 1 {
			writeSPAError(w, http.StatusForbidden, "forbidden", "CSRF validation failed")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func SPACreateOrganization(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name string `json:"name"`
	}
	if !decodeSPAJSON(w, r, &input) {
		return
	}
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" {
		writeSPAError(w, http.StatusBadRequest, "invalid_request", "Organization name required.")
		return
	}

	actor, ok := authorizeSPAInternalMutation(w, r)
	if !ok {
		return
	}
	proxyInternal(w, r, http.MethodPost, "/auth/internal/v1/organizations", map[string]string{
		"name":               input.Name,
		"created_by_user_id": actor.ID,
	})
}

func SPADeleteOrganization(w http.ResponseWriter, r *http.Request) {
	actor, ok := authorizeSPAInternalMutation(w, r)
	if !ok {
		return
	}
	proxyInternal(w, r, http.MethodDelete, "/auth/internal/v1/organizations/"+pathParam(r, "organizationID"), map[string]string{
		"actor_user_id": actor.ID,
	})
}

func SPARemoveOrganizationMember(w http.ResponseWriter, r *http.Request) {
	actor, ok := authorizeSPAInternalMutation(w, r)
	if !ok {
		return
	}
	proxyInternal(w, r, http.MethodDelete, "/auth/internal/v1/organizations/"+pathParam(r, "organizationID")+"/members/"+pathParam(r, "userID"), map[string]string{
		"actor_user_id": actor.ID,
	})
}

func SPATransferOrganizationOwnership(w http.ResponseWriter, r *http.Request) {
	var input struct {
		NewOwnerUserID string `json:"new_owner_user_id"`
	}
	if !decodeSPAJSON(w, r, &input) {
		return
	}
	input.NewOwnerUserID = strings.TrimSpace(input.NewOwnerUserID)
	if input.NewOwnerUserID == "" {
		writeSPAError(w, http.StatusBadRequest, "invalid_request", "New owner required.")
		return
	}

	actor, ok := authorizeSPAInternalMutation(w, r)
	if !ok {
		return
	}
	proxyInternal(w, r, http.MethodPost, "/auth/internal/v1/organizations/"+pathParam(r, "organizationID")+"/ownership-transfer", map[string]string{
		"actor_user_id":     actor.ID,
		"new_owner_user_id": input.NewOwnerUserID,
	})
}

func SPACreateOrganizationInvitation(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Email    string         `json:"email"`
		Role     string         `json:"role"`
		Metadata map[string]any `json:"metadata"`
	}
	if !decodeSPAJSON(w, r, &input) {
		return
	}
	input.Email = strings.TrimSpace(input.Email)
	if input.Email == "" {
		writeSPAError(w, http.StatusBadRequest, "invalid_request", "Invitation email required.")
		return
	}
	if input.Role == "" {
		input.Role = "member"
	}
	if input.Role != "member" && input.Role != "admin" {
		writeSPAError(w, http.StatusBadRequest, "invalid_request", "Invitation role must be member or admin.")
		return
	}

	actor, ok := authorizeSPAInternalMutation(w, r)
	if !ok {
		return
	}
	body := map[string]any{
		"actor_user_id": actor.ID,
		"email":         input.Email,
		"role":          input.Role,
	}
	if input.Metadata != nil {
		body["metadata"] = input.Metadata
	}
	proxyInternal(w, r, http.MethodPost, "/auth/internal/v1/organizations/"+pathParam(r, "organizationID")+"/invitations", body)
}

func SPAResendOrganizationInvitation(w http.ResponseWriter, r *http.Request) {
	if _, ok := authorizeSPAInternalMutation(w, r); !ok {
		return
	}

	organizationID := pathParam(r, "organizationID")
	invitationID := pathParam(r, "invitationID")
	// The internal resend route has no actor field. Authorize the browser user
	// through the corresponding manager-only public read before using it.
	check, err := autharaAPIResponse(r.Context(), r, http.MethodGet, "/auth/api/v1/organizations/"+organizationID+"/invitations/"+invitationID, nil)
	if err != nil {
		writeSPAError(w, http.StatusBadGateway, "upstream_error", "Authara could not be reached.")
		return
	}
	defer check.Body.Close()
	if check.StatusCode != http.StatusOK {
		copySPAResponse(w, check)
		return
	}

	proxyInternal(w, r, http.MethodPost, "/auth/internal/v1/organizations/"+organizationID+"/invitations/"+invitationID+"/resend", nil)
}

func SPADeleteCurrentUser(w http.ResponseWriter, r *http.Request) {
	actor, ok := authorizeSPAInternalMutation(w, r)
	if !ok {
		return
	}

	resp, err := internalResponse(r.Context(), http.MethodDelete, "/auth/internal/v1/users/"+url.PathEscape(actor.ID), nil)
	if err != nil {
		writeSPAError(w, http.StatusBadGateway, "upstream_error", "Authara could not be reached.")
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNoContent {
		clearSPASessionCookies(w, r)
	}
	copyInternalResponse(w, resp)
}

func authorizeSPAInternalMutation(w http.ResponseWriter, r *http.Request) (*currentUser, bool) {
	user, err := getCurrentUser(r.Context(), r)
	if err != nil {
		writeSPAError(w, http.StatusBadGateway, "upstream_error", "Authara could not be reached.")
		return nil, false
	}
	if user == nil {
		writeSPAError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized.")
		return nil, false
	}

	resp, err := autharaAPIResponseWithOptions(
		r.Context(),
		r,
		http.MethodPost,
		"/auth/api/v1/reauthenticate/check",
		nil,
		nil,
		r.Header.Get("X-CSRF-Token"),
	)
	if err != nil {
		writeSPAError(w, http.StatusBadGateway, "upstream_error", "Authara could not be reached.")
		return nil, false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		copySPAResponse(w, resp)
		return nil, false
	}
	return user, true
}

func proxyInternal(w http.ResponseWriter, r *http.Request, method, path string, body any) {
	resp, err := internalResponse(r.Context(), method, path, body)
	if err != nil {
		writeSPAError(w, http.StatusBadGateway, "upstream_error", "Authara could not be reached.")
		return
	}
	defer resp.Body.Close()
	copyInternalResponse(w, resp)
}

func copyInternalResponse(w http.ResponseWriter, resp *http.Response) {
	if resp.StatusCode == http.StatusUnauthorized {
		writeSPAError(w, http.StatusBadGateway, "internal_api_authentication_failed", "The application backend could not authenticate with Authara's internal API.")
		return
	}
	copySPAResponse(w, resp)
}

func decodeSPAJSON(w http.ResponseWriter, r *http.Request, out any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, spaAPIMaxBodyBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		writeSPAError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON body.")
		return false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeSPAError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON body.")
		return false
	}
	return true
}

func pathParam(r *http.Request, name string) string {
	return url.PathEscape(chi.URLParam(r, name))
}

func copySPAResponse(w http.ResponseWriter, resp *http.Response) {
	if contentType := resp.Header.Get("Content-Type"); contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

func writeSPAError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]string{"code": code, "message": message},
	})
}

func clearSPASessionCookies(w http.ResponseWriter, r *http.Request) {
	secure := r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
	for _, name := range []string{"authara_access", "authara_refresh"} {
		http.SetCookie(w, &http.Cookie{
			Name:     name,
			Path:     "/",
			HttpOnly: true,
			Secure:   secure,
			SameSite: http.SameSiteLaxMode,
			Expires:  time.Unix(0, 0),
			MaxAge:   -1,
		})
	}
}
