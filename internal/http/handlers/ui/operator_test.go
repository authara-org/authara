package ui

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/email"
	"github.com/authara-org/authara/internal/http/kit/httpctx"
	"github.com/authara-org/authara/internal/http/kit/render"
	"github.com/authara-org/authara/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func TestOperatorEmailTemplateSaveValidatesAndPreventsStaleOverwrite(t *testing.T) {
	templateStore := newOperatorEmailTemplateStore()
	h := newOperatorEmailTemplateHandler(templateStore)
	userID := uuid.New()

	invalid := emailTemplateFormValues(0, "Custom subject", "No code here", "<p>{{code}}</p>")
	invalidResponse := performOperatorEmailTemplateRequest(t, h.OperatorEmailTemplateSavePost, invalid, userID)
	if invalidResponse.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid save status = %d, want %d", invalidResponse.Code, http.StatusUnprocessableEntity)
	}
	if len(templateStore.overrides) != 0 {
		t.Fatal("invalid template was persisted")
	}
	if !strings.Contains(invalidResponse.Body.String(), "No code here") ||
		strings.Contains(invalidResponse.Body.String(), "Auto-render stopped") {
		t.Fatal("failed save did not retain the renderable draft preview")
	}

	valid := emailTemplateFormValues(0, "Custom subject", "Code: {{code}}", "<p>{{code}}</p>")
	savedResponse := performOperatorEmailTemplateRequest(t, h.OperatorEmailTemplateSavePost, valid, userID)
	if savedResponse.Code != http.StatusSeeOther {
		t.Fatalf("save status = %d, want %d", savedResponse.Code, http.StatusSeeOther)
	}
	assertOperatorEmailTemplateSuccessToast(t, h, savedResponse, "Email template customization saved.")
	saved := templateStore.overrides[domain.EmailTemplateSignupCode]
	if saved.Revision != 1 || saved.UpdatedByUserID == nil || *saved.UpdatedByUserID != userID {
		t.Fatalf("saved override = %#v", saved)
	}

	stale := emailTemplateFormValues(0, "Stale subject", "Code: {{code}}", "<p>{{code}}</p>")
	staleResponse := performOperatorEmailTemplateRequest(t, h.OperatorEmailTemplateSavePost, stale, userID)
	if staleResponse.Code != http.StatusConflict {
		t.Fatalf("stale save status = %d, want %d", staleResponse.Code, http.StatusConflict)
	}
	if got := templateStore.overrides[domain.EmailTemplateSignupCode].SubjectTemplate; got != "Custom subject" {
		t.Fatalf("stale save replaced subject with %q", got)
	}
	if !strings.Contains(staleResponse.Body.String(), "Another operator changed this template") {
		t.Fatal("stale response does not explain the revision conflict")
	}
}

func TestOperatorEmailTemplateHTMXSaveUpdatesRevisionAndReturnsToast(t *testing.T) {
	templateStore := newOperatorEmailTemplateStore()
	h := newOperatorEmailTemplateHandler(templateStore)
	userID := uuid.New()
	values := emailTemplateFormValues(0, "Custom subject", "Code: {{code}}", "<p>{{code}}</p>")

	response := performOperatorEmailTemplateHTMXRequest(t, h.OperatorEmailTemplateSavePost, values, userID)

	if response.Code != http.StatusOK {
		t.Fatalf("save status = %d, want %d", response.Code, http.StatusOK)
	}
	if got := response.Header().Get("HX-Trigger"); got != emailTemplatePersistedEventName {
		t.Fatalf("HX-Trigger = %q, want %q", got, emailTemplatePersistedEventName)
	}
	if got := response.Header().Get("HX-Replace-Url"); got != "/auth/operator/emails/signup_code" {
		t.Fatalf("HX-Replace-Url = %q", got)
	}
	if got := response.Header().Get("Location"); got != "" {
		t.Fatalf("HTMX save redirected to %q", got)
	}
	for _, want := range []string{
		`id="email-template-feedback"`,
		`id="email-template-source-status"`,
		`id="email-template-history"`,
		`id="email-template-revision"`,
		`id="email-template-preview"`,
		`name="revision" value="1"`,
		`id="email-template-restore-action"`,
		`id="email-template-save-action"`,
		`hx-swap-oob="outerHTML"`,
		"Customized · v1",
		"Email template customization saved.",
		`hx-swap-oob="afterbegin:#toast-container"`,
	} {
		if !strings.Contains(response.Body.String(), want) {
			t.Fatalf("HTMX save response does not contain %q", want)
		}
	}

	values.Set("revision", "1")
	response = performOperatorEmailTemplateHTMXRequest(t, h.OperatorEmailTemplateSavePost, values, userID)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `name="revision" value="2"`) {
		t.Fatalf("second HTMX save did not return revision 2: status=%d body=%s", response.Code, response.Body.String())
	}

	values.Set("revision", "1")
	response = performOperatorEmailTemplateHTMXRequest(t, h.OperatorEmailTemplateSavePost, values, userID)
	if response.Code != http.StatusConflict {
		t.Fatalf("stale HTMX save status = %d, want %d", response.Code, http.StatusConflict)
	}
	if !strings.Contains(response.Body.String(), "Another operator changed this template") ||
		strings.Contains(response.Body.String(), "data-email-template-workspace") {
		t.Fatalf("stale HTMX save did not return only the inline conflict: %s", response.Body.String())
	}
	if got := response.Header().Get("HX-Trigger"); got != "" {
		t.Fatalf("stale HTMX save triggered %q", got)
	}
}

func TestOperatorEmailTemplateHTMXSaveReturnsOnlyInlineValidationError(t *testing.T) {
	templateStore := newOperatorEmailTemplateStore()
	h := newOperatorEmailTemplateHandler(templateStore)
	values := emailTemplateFormValues(0, "Custom subject", "Code omitted", "<p>{{code}}</p>")

	response := performOperatorEmailTemplateHTMXRequest(t, h.OperatorEmailTemplateSavePost, values, uuid.New())

	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("save status = %d, want %d", response.Code, http.StatusUnprocessableEntity)
	}
	if !strings.Contains(response.Body.String(), `id="email-template-feedback"`) ||
		!strings.Contains(response.Body.String(), "requires {{code}}") {
		t.Fatalf("HTMX validation response does not contain the inline error: %s", response.Body.String())
	}
	if !strings.Contains(response.Body.String(), "The template could not be processed.") {
		t.Fatal("validation error without a source line did not retain its visible message")
	}
	if strings.Contains(response.Body.String(), "data-email-template-workspace") {
		t.Fatal("HTMX validation response replaced the complete editor")
	}
	if len(templateStore.overrides) != 0 {
		t.Fatal("invalid HTMX save persisted an override")
	}
}

func TestOperatorEmailTemplatePreviewDoesNotPersist(t *testing.T) {
	templateStore := newOperatorEmailTemplateStore()
	h := newOperatorEmailTemplateHandler(templateStore)
	userID := uuid.New()
	values := emailTemplateFormValues(0, "Preview {{code}}", "Code: {{code}}", "<strong>{{code}}</strong>")

	response := performOperatorEmailTemplateRequest(t, h.OperatorEmailTemplatePreviewPost, values, userID)

	if response.Code != http.StatusOK {
		t.Fatalf("preview status = %d, want %d", response.Code, http.StatusOK)
	}
	if len(templateStore.overrides) != 0 {
		t.Fatal("preview persisted an override")
	}
	for _, want := range []string{"Preview 123456", "Code: 123456", "sandbox=\"\""} {
		if !strings.Contains(response.Body.String(), want) {
			t.Fatalf("preview response does not contain %q", want)
		}
	}
}

func TestOperatorEmailTemplatePreviewAllowsMissingRequiredPlaceholder(t *testing.T) {
	templateStore := newOperatorEmailTemplateStore()
	h := newOperatorEmailTemplateHandler(templateStore)
	userID := uuid.New()
	values := emailTemplateFormValues(0, "Preview", "Code was omitted", "<strong>Still editing</strong>")

	response := performOperatorEmailTemplateRequest(t, h.OperatorEmailTemplatePreviewPost, values, userID)

	if response.Code != http.StatusOK {
		t.Fatalf("preview status = %d, want %d", response.Code, http.StatusOK)
	}
	for _, want := range []string{"Code was omitted", "Still editing"} {
		if !strings.Contains(response.Body.String(), want) {
			t.Fatalf("preview response does not contain %q", want)
		}
	}
	if len(templateStore.overrides) != 0 {
		t.Fatal("invalid preview persisted an override")
	}
}

func TestOperatorEmailTemplatePreviewReturnsEditorDiagnostic(t *testing.T) {
	templateStore := newOperatorEmailTemplateStore()
	h := newOperatorEmailTemplateHandler(templateStore)
	values := emailTemplateFormValues(
		0,
		"Preview",
		"Code: {{code}}",
		"<p>First line</p>\n{{code\n</p>",
	)

	response := performOperatorEmailTemplateRequest(t, h.OperatorEmailTemplatePreviewPost, values, uuid.New())

	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("preview status = %d, want %d", response.Code, http.StatusUnprocessableEntity)
	}
	for _, want := range []string{
		`data-email-template-diagnostic`,
		`data-template-part="html"`,
		`data-template-line="2"`,
		`data-template-message=`,
	} {
		if !strings.Contains(response.Body.String(), want) {
			t.Fatalf("preview diagnostic does not contain %q: %s", want, response.Body.String())
		}
	}
	if strings.Contains(response.Body.String(), "The template could not be processed.") {
		t.Fatal("line-specific diagnostic also rendered the visible error banner")
	}
}

func TestOperatorEmailTemplateResetPreventsStaleDelete(t *testing.T) {
	templateStore := newOperatorEmailTemplateStore()
	templateStore.overrides[domain.EmailTemplateSignupCode] = domain.EmailTemplateOverride{
		Template:        domain.EmailTemplateSignupCode,
		SubjectTemplate: "Custom subject",
		TextTemplate:    "Code: {{code}}",
		HTMLTemplate:    "<p>{{code}}</p>",
		Revision:        2,
	}
	h := newOperatorEmailTemplateHandler(templateStore)
	userID := uuid.New()

	staleResponse := performOperatorEmailTemplateRequest(t, h.OperatorEmailTemplateResetPost, url.Values{"revision": {"1"}}, userID)
	if staleResponse.Code != http.StatusConflict {
		t.Fatalf("stale reset status = %d, want %d", staleResponse.Code, http.StatusConflict)
	}
	if _, exists := templateStore.overrides[domain.EmailTemplateSignupCode]; !exists {
		t.Fatal("stale reset deleted the override")
	}

	resetResponse := performOperatorEmailTemplateRequest(t, h.OperatorEmailTemplateResetPost, url.Values{"revision": {"2"}}, userID)
	if resetResponse.Code != http.StatusSeeOther {
		t.Fatalf("reset status = %d, want %d", resetResponse.Code, http.StatusSeeOther)
	}
	assertOperatorEmailTemplateSuccessToast(t, h, resetResponse, "Built-in email template restored.")
	if _, exists := templateStore.overrides[domain.EmailTemplateSignupCode]; exists {
		t.Fatal("current reset did not delete the override")
	}
}

func TestOperatorEmailTemplateHTMXResetRestoresEditorAndPreview(t *testing.T) {
	templateStore := newOperatorEmailTemplateStore()
	templateStore.overrides[domain.EmailTemplateSignupCode] = domain.EmailTemplateOverride{
		Template:        domain.EmailTemplateSignupCode,
		SubjectTemplate: "Custom subject",
		TextTemplate:    "Code: {{code}}",
		HTMLTemplate:    "<p>{{code}}</p>",
		Revision:        2,
	}
	h := newOperatorEmailTemplateHandler(templateStore)

	staleResponse := performOperatorEmailTemplateHTMXRequest(
		t,
		h.OperatorEmailTemplateResetPost,
		url.Values{"revision": {"1"}},
		uuid.New(),
	)
	if staleResponse.Code != http.StatusConflict ||
		!strings.Contains(staleResponse.Body.String(), `id="email-template-feedback"`) ||
		!strings.Contains(staleResponse.Body.String(), `hx-swap-oob="outerHTML"`) {
		t.Fatalf("stale HTMX reset did not return the OOB conflict: status=%d body=%s", staleResponse.Code, staleResponse.Body.String())
	}
	if _, exists := templateStore.overrides[domain.EmailTemplateSignupCode]; !exists {
		t.Fatal("stale HTMX reset deleted the override")
	}

	response := performOperatorEmailTemplateHTMXRequest(
		t,
		h.OperatorEmailTemplateResetPost,
		url.Values{"revision": {"2"}},
		uuid.New(),
	)

	if response.Code != http.StatusOK {
		t.Fatalf("reset status = %d, want %d", response.Code, http.StatusOK)
	}
	if got := response.Header().Get("HX-Trigger"); got != emailTemplatePersistedEventName {
		t.Fatalf("HX-Trigger = %q, want %q", got, emailTemplatePersistedEventName)
	}
	if got := response.Header().Get("HX-Replace-Url"); got != "/auth/operator/emails/signup_code" {
		t.Fatalf("HX-Replace-Url = %q", got)
	}
	for _, want := range []string{
		`id="email-template-form"`,
		`id="email-template-preview"`,
		`id="email-template-source-status"`,
		`id="email-template-restore-action"`,
		`hx-swap-oob="outerHTML"`,
		`name="revision" value="0"`,
		"Built-in",
		"Built-in email template restored.",
	} {
		if !strings.Contains(response.Body.String(), want) {
			t.Fatalf("HTMX reset response does not contain %q", want)
		}
	}
	if strings.Contains(response.Body.String(), `action="/auth/operator/emails/signup_code/reset"`) {
		t.Fatal("HTMX reset response still renders the restore action")
	}
	if _, exists := templateStore.overrides[domain.EmailTemplateSignupCode]; exists {
		t.Fatal("HTMX reset did not delete the override")
	}
}

func TestOperatorEmailTemplatePreviewsHistoryWithoutSavingAndCanSaveItAsNew(t *testing.T) {
	templateStore := newOperatorEmailTemplateStore()
	h := newOperatorEmailTemplateHandler(templateStore)
	userID := uuid.New()

	firstValues := emailTemplateFormValues(0, "First subject", "First: {{code}}", "<p>First: {{code}}</p>")
	firstResponse := performOperatorEmailTemplateRequest(t, h.OperatorEmailTemplateSavePost, firstValues, userID)
	if firstResponse.Code != http.StatusSeeOther {
		t.Fatalf("first save status = %d", firstResponse.Code)
	}
	secondValues := emailTemplateFormValues(1, "Second subject", "Second: {{code}}", "<p>Second: {{code}}</p>")
	secondResponse := performOperatorEmailTemplateRequest(t, h.OperatorEmailTemplateSavePost, secondValues, userID)
	if secondResponse.Code != http.StatusSeeOther {
		t.Fatalf("second save status = %d", secondResponse.Code)
	}

	response := performOperatorEmailTemplateVersionPageRequest(t, h.OperatorEmailTemplateVersionPage, 1)
	if response.Code != http.StatusOK {
		t.Fatalf("preview version status = %d, want %d", response.Code, http.StatusOK)
	}
	current := templateStore.overrides[domain.EmailTemplateSignupCode]
	if current.Revision != 2 || current.SubjectTemplate != "Second subject" {
		t.Fatalf("preview changed current template: %#v", current)
	}
	versions := templateStore.versions[domain.EmailTemplateSignupCode]
	if len(versions) != 2 {
		t.Fatalf("preview created history: %#v", versions)
	}
	for _, want := range []string{
		`value="First subject"`,
		"First: {{code}}",
		"Viewing v1",
		"Return to current",
		"Save as new version",
		`value="2"`,
	} {
		if !strings.Contains(response.Body.String(), want) {
			t.Fatalf("historical preview does not contain %q", want)
		}
	}

	firstValues.Set("revision", "2")
	saveResponse := performOperatorEmailTemplateRequest(t, h.OperatorEmailTemplateSavePost, firstValues, userID)
	if saveResponse.Code != http.StatusSeeOther {
		t.Fatalf("save historical draft status = %d, want %d", saveResponse.Code, http.StatusSeeOther)
	}
	current = templateStore.overrides[domain.EmailTemplateSignupCode]
	if current.Revision != 3 || current.SubjectTemplate != "First subject" {
		t.Fatalf("saved historical draft = %#v", current)
	}
	versions = templateStore.versions[domain.EmailTemplateSignupCode]
	if len(versions) != 3 || versions[2].Version != 3 || versions[2].SubjectTemplate != "First subject" {
		t.Fatalf("versions after saving historical draft = %#v", versions)
	}
	assertOperatorEmailTemplateSuccessToast(t, h, saveResponse, "Email template customization saved.")
}

func assertOperatorEmailTemplateSuccessToast(
	t *testing.T,
	h *UIHandler,
	redirectResponse *httptest.ResponseRecorder,
	wantMessage string,
) {
	t.Helper()

	location := redirectResponse.Header().Get("Location")
	wantLocation := "/auth/operator/emails/" + string(domain.EmailTemplateSignupCode)
	if location != wantLocation {
		t.Fatalf("redirect location = %q, want %q", location, wantLocation)
	}

	var flashCookie *http.Cookie
	for _, cookie := range redirectResponse.Result().Cookies() {
		if cookie.Name == "authara_flash" {
			flashCookie = cookie
			break
		}
	}
	if flashCookie == nil {
		t.Fatal("success redirect did not set a flash cookie")
	}

	req := httptest.NewRequest(http.MethodGet, location, nil)
	req.AddCookie(flashCookie)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("templateKey", string(domain.EmailTemplateSignupCode))
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))

	pageResponse := httptest.NewRecorder()
	h.OperatorEmailTemplatePage(pageResponse, req)
	if pageResponse.Code != http.StatusOK {
		t.Fatalf("redirected page status = %d, want %d", pageResponse.Code, http.StatusOK)
	}
	for _, want := range []string{"Success!", wantMessage, "toast-container"} {
		if !strings.Contains(pageResponse.Body.String(), want) {
			t.Fatalf("redirected page does not contain toast content %q", want)
		}
	}
}

func newOperatorEmailTemplateHandler(templateStore *operatorEmailTemplateStore) *UIHandler {
	return &UIHandler{
		EmailTemplates: email.NewTemplateService(templateStore),
		Logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
		Render:         render.New(render.Assets{}, false),
	}
}

func performOperatorEmailTemplateRequest(
	t *testing.T,
	handler http.HandlerFunc,
	values url.Values,
	userID uuid.UUID,
) *httptest.ResponseRecorder {
	return performOperatorEmailTemplateRequestMode(t, handler, values, userID, false)
}

func performOperatorEmailTemplateHTMXRequest(
	t *testing.T,
	handler http.HandlerFunc,
	values url.Values,
	userID uuid.UUID,
) *httptest.ResponseRecorder {
	return performOperatorEmailTemplateRequestMode(t, handler, values, userID, true)
}

func performOperatorEmailTemplateVersionPageRequest(
	t *testing.T,
	handler http.HandlerFunc,
	version int64,
) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/auth/operator/emails/signup_code/versions/"+strconv.FormatInt(version, 10), nil)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("templateKey", string(domain.EmailTemplateSignupCode))
	routeCtx.URLParams.Add("version", strconv.FormatInt(version, 10))
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))

	response := httptest.NewRecorder()
	handler(response, req)
	return response
}

func performOperatorEmailTemplateRequestMode(
	t *testing.T,
	handler http.HandlerFunc,
	values url.Values,
	userID uuid.UUID,
	isHTMX bool,
) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/auth/operator/emails/signup_code", strings.NewReader(values.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if isHTMX {
		req.Header.Set("HX-Request", "true")
	}
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("templateKey", string(domain.EmailTemplateSignupCode))
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx)
	ctx = httpctx.WithUserID(ctx, userID)
	if isHTMX {
		ctx = httpctx.WithHTMX(ctx)
	}
	req = req.WithContext(ctx)

	response := httptest.NewRecorder()
	handler(response, req)
	return response
}

func emailTemplateFormValues(revision int64, subject, textBody, htmlBody string) url.Values {
	return url.Values{
		"revision":         {strconv.FormatInt(revision, 10)},
		"subject_template": {subject},
		"text_template":    {textBody},
		"html_template":    {htmlBody},
	}
}

type operatorEmailTemplateStore struct {
	overrides map[domain.EmailTemplate]domain.EmailTemplateOverride
	versions  map[domain.EmailTemplate][]domain.EmailTemplateVersion
}

func newOperatorEmailTemplateStore() *operatorEmailTemplateStore {
	return &operatorEmailTemplateStore{
		overrides: make(map[domain.EmailTemplate]domain.EmailTemplateOverride),
		versions:  make(map[domain.EmailTemplate][]domain.EmailTemplateVersion),
	}
}

func (s *operatorEmailTemplateStore) GetEmailTemplateOverride(_ context.Context, key domain.EmailTemplate) (domain.EmailTemplateOverride, error) {
	override, ok := s.overrides[key]
	if !ok {
		return domain.EmailTemplateOverride{}, store.ErrEmailTemplateOverrideNotFound
	}
	return override, nil
}

func (s *operatorEmailTemplateStore) ListEmailTemplateOverrides(context.Context) ([]domain.EmailTemplateOverride, error) {
	overrides := make([]domain.EmailTemplateOverride, 0, len(s.overrides))
	for _, override := range s.overrides {
		overrides = append(overrides, override)
	}
	return overrides, nil
}

func (s *operatorEmailTemplateStore) GetEmailTemplateVersion(_ context.Context, key domain.EmailTemplate, version int64) (domain.EmailTemplateVersion, error) {
	for _, candidate := range s.versions[key] {
		if candidate.Version == version {
			return candidate, nil
		}
	}
	return domain.EmailTemplateVersion{}, store.ErrEmailTemplateVersionNotFound
}

func (s *operatorEmailTemplateStore) ListEmailTemplateVersions(_ context.Context, key domain.EmailTemplate) ([]domain.EmailTemplateVersion, error) {
	versions := s.versions[key]
	out := make([]domain.EmailTemplateVersion, len(versions))
	for i := range versions {
		out[len(versions)-1-i] = versions[i]
	}
	return out, nil
}

func (s *operatorEmailTemplateStore) UpsertEmailTemplateOverride(_ context.Context, override domain.EmailTemplateOverride, expectedRevision int64) (domain.EmailTemplateOverride, error) {
	current, exists := s.overrides[override.Template]
	if (!exists && expectedRevision != 0) || (exists && current.Revision != expectedRevision) {
		return domain.EmailTemplateOverride{}, store.ErrEmailTemplateRevisionConflict
	}
	override.Revision = current.Revision + 1
	s.overrides[override.Template] = override
	history := s.versions[override.Template]
	s.versions[override.Template] = append(history, domain.EmailTemplateVersion{
		Template:        override.Template,
		Version:         int64(len(history) + 1),
		SubjectTemplate: override.SubjectTemplate,
		TextTemplate:    override.TextTemplate,
		HTMLTemplate:    override.HTMLTemplate,
		CreatedByUserID: override.UpdatedByUserID,
	})
	return override, nil
}

func (s *operatorEmailTemplateStore) DeleteEmailTemplateOverride(_ context.Context, key domain.EmailTemplate, expectedRevision int64) error {
	current, exists := s.overrides[key]
	if !exists || current.Revision != expectedRevision {
		return store.ErrEmailTemplateRevisionConflict
	}
	delete(s.overrides, key)
	return nil
}

var _ email.TemplateOverrideStore = (*operatorEmailTemplateStore)(nil)
