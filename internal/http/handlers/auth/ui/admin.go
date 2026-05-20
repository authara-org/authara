package ui

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/a-h/templ"
	adminsvc "github.com/authara-org/authara/internal/admin"
	"github.com/authara-org/authara/internal/http/kit/httpctx"
	"github.com/authara-org/authara/internal/http/kit/httputil"
	adminview "github.com/authara-org/authara/internal/http/templates/admin"
	"github.com/authara-org/authara/internal/http/templates/components/toast"
	"github.com/authara-org/authara/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func (h *UIHandler) AdminPage(w http.ResponseWriter, r *http.Request) {
	stats, err := h.Admin.DashboardStats(r.Context())
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}

	failures, err := h.Admin.RecentFailures(r.Context(), adminsvc.Page{Page: 1, Size: 5})
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}

	h.Render(w, r, http.StatusOK, adminview.Dashboard(stats, failures, h.adminFeatures()))
}

func (h *UIHandler) AdminUsersPage(w http.ResponseWriter, r *http.Request) {
	h.Render(w, r, http.StatusOK, adminview.Users(r.URL.Query().Get("q"), h.adminFeatures()))
}

func (h *UIHandler) AdminUserSearchGet(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if query == "" {
		query = strings.TrimSpace(r.URL.Query().Get("email"))
	}
	if query == "" {
		h.Render(
			w,
			r,
			http.StatusOK,
			templ.Join(
				adminview.UserSearchResult(nil),
				toast.ToastMessage(toast.Error, "Enter an email address or username."),
			),
		)
		return
	}

	result, err := h.Admin.SearchUser(r.Context(), query)
	if err != nil {
		if errors.Is(err, store.ErrUserNotFound) {
			h.Render(
				w,
				r,
				http.StatusOK,
				templ.Join(
					adminview.UserSearchResult(nil),
					toast.ToastMessage(toast.Info, "No user found for that email or username."),
				),
			)
			return
		}
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}

	h.Render(
		w,
		r,
		http.StatusOK,
		templ.Join(
			adminview.UserSearchResult(&result),
			toast.ToastMessage(toast.Success, "User found."),
		),
	)
}

func (h *UIHandler) AdminUserDetailPage(w http.ResponseWriter, r *http.Request) {
	userID, ok := parseUUIDParam(w, r, "userID")
	if !ok {
		return
	}
	actor, ok := adminActorFromRequest(w, r)
	if !ok {
		return
	}

	detail, err := h.Admin.GetUserDetail(r.Context(), actor, userID)
	if err != nil {
		if errors.Is(err, store.ErrUserNotFound) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}

	h.Render(w, r, http.StatusOK, adminview.UserDetail(detail, h.adminFeatures()))
}

func (h *UIHandler) DisableUserPost(w http.ResponseWriter, r *http.Request) {
	h.mutateUser(w, r, "User disabled.", func(actor adminsvc.Actor, userID uuid.UUID, meta adminsvc.RequestMeta) error {
		return h.Admin.DisableUser(r.Context(), actor, userID, meta)
	})
}

func (h *UIHandler) EnableUserPost(w http.ResponseWriter, r *http.Request) {
	h.mutateUser(w, r, "User enabled.", func(actor adminsvc.Actor, userID uuid.UUID, meta adminsvc.RequestMeta) error {
		return h.Admin.EnableUser(r.Context(), actor, userID, meta)
	})
}

func (h *UIHandler) GrantAdminPost(w http.ResponseWriter, r *http.Request) {
	h.mutateUser(w, r, "Admin role granted.", func(actor adminsvc.Actor, userID uuid.UUID, meta adminsvc.RequestMeta) error {
		return h.Admin.GrantAdmin(r.Context(), actor, userID, meta)
	})
}

func (h *UIHandler) RevokeAdminPost(w http.ResponseWriter, r *http.Request) {
	h.mutateUser(w, r, "Admin role removed.", func(actor adminsvc.Actor, userID uuid.UUID, meta adminsvc.RequestMeta) error {
		return h.Admin.RevokeAdmin(r.Context(), actor, userID, meta)
	})
}

func (h *UIHandler) RevokeAdminUserSessionPost(w http.ResponseWriter, r *http.Request) {
	userID, ok := parseUUIDParam(w, r, "userID")
	if !ok {
		return
	}
	sessionID, ok := parseUUIDParam(w, r, "sessionID")
	if !ok {
		return
	}
	actor, ok := adminActorFromRequest(w, r)
	if !ok {
		return
	}

	err := h.Admin.RevokeUserSession(r.Context(), actor, userID, sessionID, requestMeta(r))
	h.renderUserMutationResult(w, r, actor, userID, "Session revoked.", err)
}

func (h *UIHandler) RevokeAllAdminUserSessionsPost(w http.ResponseWriter, r *http.Request) {
	h.mutateUser(w, r, "Active sessions revoked.", func(actor adminsvc.Actor, userID uuid.UUID, meta adminsvc.RequestMeta) error {
		return h.Admin.RevokeAllUserSessions(r.Context(), actor, userID, meta)
	})
}

func (h *UIHandler) AdminAllowlistPage(w http.ResponseWriter, r *http.Request) {
	if !h.Features.AllowlistEnabled {
		http.NotFound(w, r)
		return
	}

	h.Render(w, r, http.StatusOK, adminview.Allowlist(strings.TrimSpace(r.URL.Query().Get("q")), h.adminFeatures()))
}

func (h *UIHandler) AdminAllowlistResultsGet(w http.ResponseWriter, r *http.Request) {
	if !h.Features.AllowlistEnabled {
		http.NotFound(w, r)
		return
	}

	page, err := h.Admin.ListAllowedEmails(r.Context(), r.URL.Query().Get("q"), pageFromRequest(r, 25))
	if err != nil {
		if errors.Is(err, adminsvc.ErrAllowlistDisabled) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}

	h.Render(w, r, http.StatusOK, adminview.AllowlistResults(page))
}

func (h *UIHandler) AdminAllowlistCreatePost(w http.ResponseWriter, r *http.Request) {
	if !h.Features.AllowlistEnabled {
		http.NotFound(w, r)
		return
	}

	actor, ok := adminActorFromRequest(w, r)
	if !ok {
		return
	}
	err := h.Admin.AddAllowedEmail(r.Context(), actor, r.FormValue("email"), requestMeta(r))
	h.renderAllowlistMutationResult(w, r, "Allowlisted email added.", err)
}

func (h *UIHandler) AdminAllowlistDeletePost(w http.ResponseWriter, r *http.Request) {
	if !h.Features.AllowlistEnabled {
		http.NotFound(w, r)
		return
	}

	emailID, ok := parseUUIDParam(w, r, "emailID")
	if !ok {
		return
	}
	actor, ok := adminActorFromRequest(w, r)
	if !ok {
		return
	}
	err := h.Admin.RemoveAllowedEmail(r.Context(), actor, emailID, requestMeta(r))
	h.renderAllowlistMutationResult(w, r, "Allowlisted email removed.", err)
}

func (h *UIHandler) AdminFailuresPage(w http.ResponseWriter, r *http.Request) {
	failures, err := h.Admin.RecentFailures(r.Context(), pageFromRequest(r, 25))
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}

	h.Render(w, r, http.StatusOK, adminview.Failures(failures, h.adminFeatures()))
}

func (h *UIHandler) AdminAuditPage(w http.ResponseWriter, r *http.Request) {
	events, err := h.Admin.ListAuditEvents(r.Context(), pageFromRequest(r, 50))
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}

	h.Render(w, r, http.StatusOK, adminview.Audit(events, h.adminFeatures()))
}

func (h *UIHandler) mutateUser(
	w http.ResponseWriter,
	r *http.Request,
	success string,
	fn func(adminsvc.Actor, uuid.UUID, adminsvc.RequestMeta) error,
) {
	userID, ok := parseUUIDParam(w, r, "userID")
	if !ok {
		return
	}
	actor, ok := adminActorFromRequest(w, r)
	if !ok {
		return
	}

	err := fn(actor, userID, requestMeta(r))
	h.renderUserMutationResult(w, r, actor, userID, success, err)
}

func (h *UIHandler) renderUserMutationResult(w http.ResponseWriter, r *http.Request, actor adminsvc.Actor, userID uuid.UUID, success string, err error) {
	if err != nil {
		detail, detailErr := h.Admin.GetUserDetail(r.Context(), actor, userID)
		if detailErr != nil {
			http.Error(w, adminErrorMessage(err), http.StatusBadRequest)
			return
		}
		h.Render(
			w,
			r,
			http.StatusBadRequest,
			templ.Join(
				adminview.UserDetail(detail, h.adminFeatures()),
				toast.ToastMessage(toast.Error, adminErrorMessage(err)),
			),
		)
		return
	}

	detail, err := h.Admin.GetUserDetail(r.Context(), actor, userID)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	h.Render(
		w,
		r,
		http.StatusOK,
		templ.Join(
			adminview.UserDetail(detail, h.adminFeatures()),
			toast.ToastMessage(toast.Success, success),
		),
	)
}

func (h *UIHandler) renderAllowlistMutationResult(w http.ResponseWriter, r *http.Request, success string, err error) {
	if errors.Is(err, adminsvc.ErrAllowlistDisabled) {
		http.NotFound(w, r)
		return
	}

	kind := toast.Success
	message := success
	status := http.StatusOK
	if err != nil {
		kind = toast.Error
		message = adminErrorMessage(err)
		status = http.StatusBadRequest
	}

	page, pageErr := h.Admin.ListAllowedEmails(r.Context(), allowlistQueryFromRequest(r), pageFromRequest(r, 25))
	if pageErr != nil {
		if errors.Is(pageErr, adminsvc.ErrAllowlistDisabled) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	h.Render(
		w,
		r,
		status,
		templ.Join(
			adminview.AllowlistResults(page),
			toast.ToastMessage(kind, message),
		),
	)
}

func (h *UIHandler) adminFeatures() adminview.FeatureFlags {
	return h.Features
}

func allowlistQueryFromRequest(r *http.Request) string {
	if value := strings.TrimSpace(r.FormValue("q")); value != "" {
		return value
	}
	return strings.TrimSpace(r.URL.Query().Get("q"))
}

func parseUUIDParam(w http.ResponseWriter, r *http.Request, name string) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, name))
	if err != nil {
		http.Error(w, "invalid "+name, http.StatusBadRequest)
		return uuid.Nil, false
	}
	return id, true
}

func adminActorFromRequest(w http.ResponseWriter, r *http.Request) (adminsvc.Actor, bool) {
	userID, ok := httpctx.UserID(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return adminsvc.Actor{}, false
	}
	actorRoles, _ := httpctx.Roles(r.Context())
	return adminsvc.Actor{UserID: userID, Roles: actorRoles}, true
}

func requestMeta(r *http.Request) adminsvc.RequestMeta {
	ip := ""
	if parsed := httputil.ClientIP(r); parsed != nil {
		ip = parsed.String()
	}
	return adminsvc.RequestMeta{
		IP:        ip,
		UserAgent: r.UserAgent(),
	}
}

func pageFromRequest(r *http.Request, defaultSize int) adminsvc.Page {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	size, _ := strconv.Atoi(r.URL.Query().Get("size"))
	if size <= 0 {
		size = defaultSize
	}
	return adminsvc.Page{Page: page, Size: size}
}

func adminErrorMessage(err error) string {
	switch {
	case errors.Is(err, adminsvc.ErrSelfDisable):
		return "You cannot disable your own account."
	case errors.Is(err, adminsvc.ErrSelfRevokeAdmin):
		return "You cannot remove your own admin role."
	case errors.Is(err, adminsvc.ErrSelfRevokeSessions):
		return "You cannot revoke all sessions for your own account from here."
	case errors.Is(err, adminsvc.ErrLastAdmin):
		return "This action would leave Authara without an active admin."
	case errors.Is(err, adminsvc.ErrAllowlistDisabled):
		return "Allowlist management is not enabled."
	case errors.Is(err, adminsvc.ErrAllowedEmailAlreadyAdded):
		return "That email is already allowlisted."
	case errors.Is(err, adminsvc.ErrInvalidEmail):
		return "Enter a valid email address."
	case errors.Is(err, store.ErrUserNotFound):
		return "User not found."
	case errors.Is(err, store.ErrSessionNotFound):
		return "Session not found."
	case errors.Is(err, store.ErrAllowedEmailNotFound):
		return "Allowlisted email not found."
	default:
		return "The admin action could not be completed."
	}
}
