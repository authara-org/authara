package ui

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/a-h/templ"
	"github.com/authara-org/authara/internal/config"
	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/email"
	"github.com/authara-org/authara/internal/http/kit/flash"
	"github.com/authara-org/authara/internal/http/kit/htmx"
	"github.com/authara-org/authara/internal/http/kit/httpctx"
	"github.com/authara-org/authara/internal/http/templates/components/toast"
	operatorview "github.com/authara-org/authara/internal/http/templates/operator"
	"github.com/authara-org/authara/internal/store"
	"github.com/go-chi/chi/v5"
)

const (
	maxEmailTemplateFormBytes       = 2 << 20
	emailTemplatePersistedEventName = "email-template-persisted"
)

type emailTemplateForm struct {
	SubjectTemplate string
	TextTemplate    string
	HTMLTemplate    string
	Revision        int64
}

func (h *UIHandler) OperatorPage(w http.ResponseWriter, r *http.Request) {
	_ = h.Render(w, r, http.StatusOK, operatorview.Dashboard(len(email.TemplateCatalog())))
}

func (h *UIHandler) OperatorSettingsPage(w http.ResponseWriter, r *http.Request) {
	msg, _ := flash.Read(w, r)
	if msg != nil {
		r = r.WithContext(httpctx.WithFlash(r.Context(), msg))
	}
	if h.Config == nil {
		h.renderInternalError(w, r)
		return
	}
	_ = h.Render(w, r, http.StatusOK, operatorview.Settings(operatorview.SettingsPageModel{
		Settings: h.Config.List(),
	}))
}

func (h *UIHandler) OperatorSettingSetPost(w http.ResponseWriter, r *http.Request) {
	h.mutateOperatorSetting(w, r, false)
}

func (h *UIHandler) OperatorSettingClearPost(w http.ResponseWriter, r *http.Request) {
	h.mutateOperatorSetting(w, r, true)
}

func (h *UIHandler) mutateOperatorSetting(w http.ResponseWriter, r *http.Request, clear bool) {
	if h.Config == nil {
		h.renderInternalError(w, r)
		return
	}
	key := config.Key(chi.URLParam(r, "settingKey"))
	if _, err := h.Config.Describe(key); errors.Is(err, config.ErrUnknownSetting) {
		h.renderNotFound(w, r)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	if err := r.ParseForm(); err != nil {
		h.renderRequestError(w, r, http.StatusBadRequest, "Invalid runtime setting form.")
		return
	}
	expectedRevision, err := strconv.ParseInt(r.Form.Get("revision"), 10, 64)
	if err != nil || expectedRevision < 0 {
		h.renderRequestError(w, r, http.StatusBadRequest, "Invalid runtime setting revision.")
		return
	}
	userID, ok := httpctx.UserID(r.Context())
	if !ok {
		h.renderUnauthorized(w, r)
		return
	}
	var description config.Description
	if clear {
		description, err = h.Config.Clear(r.Context(), key, userID, expectedRevision)
	} else {
		description, err = h.Config.Set(r.Context(), key, r.Form.Get("value"), userID, expectedRevision)
	}
	if err == nil {
		message := "Runtime setting updated. New operations now use the new value."
		if clear {
			message = "Operator override cleared. New operations now use the resolved fallback value."
		}
		if httpctx.IsHTMX(r.Context()) {
			_ = h.Render(w, r, http.StatusOK, templ.Join(
				operatorview.RuntimeSettingRow(description, description.EffectiveValue, ""),
				toast.ToastMessage(toast.Success, message),
			))
			return
		}
		_ = flash.Set(w, flash.Message{Kind: "success", Message: message})
		http.Redirect(w, r, "/auth/operator/settings", http.StatusSeeOther)
		return
	}

	status := http.StatusUnprocessableEntity
	message := err.Error()
	switch {
	case errors.Is(err, config.ErrUnknownSetting):
		h.renderNotFound(w, r)
		return
	case errors.Is(err, config.ErrSettingLocked):
		status = http.StatusForbidden
		message = "This setting is managed by deployment configuration and cannot be changed here."
	case errors.Is(err, config.ErrRevisionConflict):
		status = http.StatusConflict
		message = "Another operator changed runtime config. Review the current values and try again."
	case !errors.Is(err, config.ErrInvalidValue):
		if h.Logger != nil {
			h.Logger.Error("mutate runtime setting", "key", key, "err", err)
		}
		h.renderInternalError(w, r)
		return
	}
	if httpctx.IsHTMX(r.Context()) {
		current, describeErr := h.Config.Describe(key)
		if describeErr != nil {
			if h.Logger != nil {
				h.Logger.Error("describe runtime setting after failed mutation", "key", key, "err", describeErr)
			}
			h.renderInternalError(w, r)
			return
		}
		inputValue := current.EffectiveValue
		if !clear {
			inputValue = r.Form.Get("value")
		}
		_ = h.Render(w, r, status, templ.Join(
			operatorview.RuntimeSettingRow(current, inputValue, message),
			toast.ToastMessage(toast.Error, message),
		))
		return
	}
	var draftValue *string
	if !clear {
		value := r.Form.Get("value")
		draftValue = &value
	}
	_ = h.Render(w, r, status, operatorview.Settings(operatorview.SettingsPageModel{
		Settings: h.Config.List(), ErrorKey: key, Error: message, DraftValue: draftValue,
	}))
}

func (h *UIHandler) OperatorAuditPage(w http.ResponseWriter, r *http.Request) {
	requestedPage := pageFromRequest(r, 50)
	action := r.URL.Query().Get("action")
	if action != domain.OperatorAuditActionEmailTemplateSaved &&
		action != domain.OperatorAuditActionEmailTemplateRestoredBuiltIn &&
		action != domain.OperatorAuditActionEmailTemplateDeliveryEnabled &&
		action != domain.OperatorAuditActionEmailTemplateDeliveryDisabled &&
		action != domain.OperatorAuditActionRuntimeSettingSet &&
		action != domain.OperatorAuditActionRuntimeSettingCleared {
		action = ""
	}
	template := domain.EmailTemplate(r.URL.Query().Get("template"))
	if template != "" {
		if err := email.ValidateTemplate(template); err != nil {
			template = ""
		}
	}

	page, err := h.EmailTemplates.ListAuditEvents(r.Context(), email.OperatorAuditQuery{
		Page:     requestedPage.Page,
		Size:     requestedPage.Size,
		Action:   action,
		Template: template,
	})
	if err != nil {
		h.logEmailTemplateError("list operator audit events", err)
		h.renderInternalError(w, r)
		return
	}
	_ = h.Render(w, r, http.StatusOK, operatorview.Audit(page))
}

func (h *UIHandler) OperatorEmailTemplatesPage(w http.ResponseWriter, r *http.Request) {
	msg, _ := flash.Read(w, r)
	if msg != nil {
		r = r.WithContext(httpctx.WithFlash(r.Context(), msg))
	}
	templates, err := h.EmailTemplates.List(r.Context())
	if err != nil {
		h.logEmailTemplateError("list operator email templates", err)
		h.renderInternalError(w, r)
		return
	}
	_ = h.Render(w, r, http.StatusOK, operatorview.EmailTemplates(templates))
}

func (h *UIHandler) OperatorEmailTemplateDeliveryPost(w http.ResponseWriter, r *http.Request) {
	key, ok := h.operatorEmailTemplateKey(w, r)
	if !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		h.renderRequestError(w, r, http.StatusBadRequest, "Invalid email delivery setting.")
		return
	}
	enabled, err := strconv.ParseBool(r.Form.Get("enabled"))
	if err != nil {
		h.renderRequestError(w, r, http.StatusBadRequest, "Invalid email delivery setting.")
		return
	}
	userID, ok := httpctx.UserID(r.Context())
	if !ok {
		h.renderUnauthorized(w, r)
		return
	}
	if err := h.EmailTemplates.SetDeliveryEnabled(r.Context(), key, enabled, userID); err != nil {
		h.logEmailTemplateError("set operator email template delivery", err)
		h.renderInternalError(w, r)
		return
	}
	message := "Email delivery disabled."
	if enabled {
		message = "Email delivery enabled."
	}
	if httpctx.IsHTMX(r.Context()) {
		_ = h.Render(w, r, http.StatusOK, templ.Join(
			operatorview.EmailTemplateDeliveryToggle(key, enabled),
			toast.ToastMessage(toast.Success, message),
		))
		return
	}
	_ = flash.Set(w, flash.Message{Kind: "success", Message: message})
	http.Redirect(w, r, "/auth/operator/emails", http.StatusSeeOther)
}

func (h *UIHandler) OperatorEmailTemplatePage(w http.ResponseWriter, r *http.Request) {
	msg, _ := flash.Read(w, r)
	if msg != nil {
		r = r.WithContext(httpctx.WithFlash(r.Context(), msg))
	}

	effective, ok := h.operatorEmailTemplate(w, r)
	if !ok {
		return
	}

	model, err := h.emailTemplateEditorModel(r.Context(), effective)
	if err != nil {
		h.logEmailTemplateError("render operator email template", err)
		h.renderInternalError(w, r)
		return
	}
	_ = h.Render(w, r, http.StatusOK, operatorview.EmailTemplateEditor(model))
}

func (h *UIHandler) OperatorEmailTemplateVersionPage(w http.ResponseWriter, r *http.Request) {
	effective, ok := h.operatorEmailTemplate(w, r)
	if !ok {
		return
	}
	version, err := strconv.ParseInt(chi.URLParam(r, "version"), 10, 64)
	if err != nil || version <= 0 {
		h.renderNotFound(w, r)
		return
	}

	snapshot, err := h.EmailTemplates.GetVersion(r.Context(), effective.Definition.Key, version)
	if errors.Is(err, store.ErrEmailTemplateVersionNotFound) {
		h.renderNotFound(w, r)
		return
	}
	if err != nil {
		h.logEmailTemplateError("get operator email template version", err)
		h.renderInternalError(w, r)
		return
	}

	model, err := h.emailTemplateEditorModel(r.Context(), effective)
	if err != nil {
		h.logEmailTemplateError("render operator email template version", err)
		h.renderInternalError(w, r)
		return
	}
	model.ViewingVersion = snapshot.Version
	model.SubjectTemplate = snapshot.SubjectTemplate
	model.TextTemplate = snapshot.TextTemplate
	model.HTMLTemplate = snapshot.HTMLTemplate
	preview, previewErr := h.EmailTemplates.Preview(email.PreviewTemplateInput{
		Template:        snapshot.Template,
		SubjectTemplate: snapshot.SubjectTemplate,
		TextTemplate:    snapshot.TextTemplate,
		HTMLTemplate:    snapshot.HTMLTemplate,
	})
	if previewErr != nil {
		model.Preview = nil
		setEmailTemplateEditorError(&model, previewErr)
	} else {
		model.Preview = &preview
	}

	_ = h.Render(w, r, http.StatusOK, operatorview.EmailTemplateEditor(model))
}

func (h *UIHandler) OperatorEmailTemplatePreviewPost(w http.ResponseWriter, r *http.Request) {
	effective, ok := h.operatorEmailTemplate(w, r)
	if !ok {
		return
	}
	form, ok := h.parseEmailTemplateForm(w, r)
	if !ok {
		return
	}

	model := emailTemplateEditorModelFromForm(effective, form)
	preview, err := h.EmailTemplates.Preview(email.PreviewTemplateInput{
		Template:        effective.Definition.Key,
		SubjectTemplate: form.SubjectTemplate,
		TextTemplate:    form.TextTemplate,
		HTMLTemplate:    form.HTMLTemplate,
	})
	status := http.StatusOK
	if err != nil {
		setEmailTemplateEditorError(&model, err)
		status = http.StatusUnprocessableEntity
	} else {
		model.Preview = &preview
	}
	_ = h.Render(w, r, status, operatorview.EmailTemplatePreviewResponse(model))
}

func (h *UIHandler) OperatorEmailTemplateSavePost(w http.ResponseWriter, r *http.Request) {
	key, ok := h.operatorEmailTemplateKey(w, r)
	if !ok {
		return
	}
	form, ok := h.parseEmailTemplateForm(w, r)
	if !ok {
		return
	}
	userID, ok := httpctx.UserID(r.Context())
	if !ok {
		h.renderUnauthorized(w, r)
		return
	}

	saved, err := h.EmailTemplates.SaveOverride(r.Context(), email.SaveTemplateOverrideInput{
		Template:         key,
		SubjectTemplate:  form.SubjectTemplate,
		TextTemplate:     form.TextTemplate,
		HTMLTemplate:     form.HTMLTemplate,
		ExpectedRevision: form.Revision,
		UpdatedByUserID:  userID,
	})
	if err == nil {
		if httpctx.IsHTMX(r.Context()) {
			if saved.Override == nil {
				h.logEmailTemplateError("render saved operator email template", errors.New("saved email template override is missing"))
				h.renderInternalError(w, r)
				return
			}
			model := emailTemplateEditorModelFromForm(saved, form)
			model.Revision = saved.Override.Revision
			if err := h.populateEmailTemplateHistory(r.Context(), &model); err != nil {
				h.logEmailTemplateError("load saved operator email template history", err)
				h.renderInternalError(w, r)
				return
			}
			preview, previewErr := h.EmailTemplates.Preview(email.PreviewTemplateInput{
				Template:        saved.Definition.Key,
				SubjectTemplate: form.SubjectTemplate,
				TextTemplate:    form.TextTemplate,
				HTMLTemplate:    form.HTMLTemplate,
			})
			if previewErr != nil {
				h.logEmailTemplateError("render saved operator email template preview", previewErr)
				h.renderInternalError(w, r)
				return
			}
			model.Preview = &preview
			htmx.ReplaceUrl(w, "/auth/operator/emails/"+string(saved.Definition.Key))
			h.renderEmailTemplateMutationSuccess(
				w,
				r,
				operatorview.EmailTemplateSaveSuccess(model),
				"Email template customization saved.",
			)
			return
		}
		h.redirectEmailTemplate(w, r, saved.Definition.Key, "Email template customization saved.")
		return
	}

	definition, lookupErr := email.LookupTemplate(key)
	if lookupErr != nil {
		h.renderNotFound(w, r)
		return
	}
	model := emailTemplateEditorModelFromForm(email.EffectiveTemplate{
		Definition: definition,
		Source:     email.TemplateSourceBuiltIn,
	}, form)
	if form.Revision > 0 {
		model.Source = email.TemplateSourceOverride
	}
	preview, previewErr := h.EmailTemplates.Preview(email.PreviewTemplateInput{
		Template:        key,
		SubjectTemplate: form.SubjectTemplate,
		TextTemplate:    form.TextTemplate,
		HTMLTemplate:    form.HTMLTemplate,
	})
	if previewErr == nil {
		model.Preview = &preview
	}

	status := http.StatusUnprocessableEntity
	if errors.Is(err, store.ErrEmailTemplateRevisionConflict) {
		status = http.StatusConflict
		model.Conflict = true
		model.Error = "Another operator changed this template after you opened it. Reload the current revision before saving your changes."
	} else if isEmailTemplateValidationError(err) {
		setEmailTemplateEditorError(&model, err)
	} else {
		h.logEmailTemplateError("save operator email template", err)
		h.renderInternalError(w, r)
		return
	}
	if httpctx.IsHTMX(r.Context()) {
		_ = h.Render(w, r, status, operatorview.EmailTemplateFeedback(model))
		return
	}
	if err := h.populateEmailTemplateHistory(r.Context(), &model); err != nil {
		h.logEmailTemplateError("load operator email template history after failed save", err)
		h.renderInternalError(w, r)
		return
	}
	_ = h.Render(w, r, status, operatorview.EmailTemplateEditor(model))
}

func (h *UIHandler) OperatorEmailTemplateResetPost(w http.ResponseWriter, r *http.Request) {
	key, ok := h.operatorEmailTemplateKey(w, r)
	if !ok {
		return
	}
	form, ok := h.parseEmailTemplateRevision(w, r)
	if !ok {
		return
	}
	userID, ok := httpctx.UserID(r.Context())
	if !ok {
		h.renderUnauthorized(w, r)
		return
	}

	err := h.EmailTemplates.DeleteOverride(r.Context(), key, form, userID)
	if err == nil {
		if httpctx.IsHTMX(r.Context()) {
			effective, getErr := h.EmailTemplates.Get(r.Context(), key)
			if getErr != nil {
				h.logEmailTemplateError("reload restored operator email template", getErr)
				h.renderInternalError(w, r)
				return
			}
			model, modelErr := h.emailTemplateEditorModel(r.Context(), effective)
			if modelErr != nil {
				h.logEmailTemplateError("render restored operator email template", modelErr)
				h.renderInternalError(w, r)
				return
			}
			htmx.ReplaceUrl(w, "/auth/operator/emails/"+string(key))
			h.renderEmailTemplateMutationSuccess(
				w,
				r,
				operatorview.EmailTemplateRestoreSuccess(model),
				"Built-in email template restored.",
			)
			return
		}
		h.redirectEmailTemplate(w, r, key, "Built-in email template restored.")
		return
	}
	if errors.Is(err, store.ErrEmailTemplateRevisionConflict) {
		effective, getErr := h.EmailTemplates.Get(r.Context(), key)
		if getErr != nil {
			h.logEmailTemplateError("reload conflicted operator email template", getErr)
			h.renderInternalError(w, r)
			return
		}
		model, modelErr := h.emailTemplateEditorModel(r.Context(), effective)
		if modelErr != nil {
			h.logEmailTemplateError("render conflicted operator email template", modelErr)
			h.renderInternalError(w, r)
			return
		}
		model.Conflict = true
		model.Error = "Another operator changed this template before it could be restored. Review the current revision and try again."
		if httpctx.IsHTMX(r.Context()) {
			_ = h.Render(w, r, http.StatusConflict, operatorview.EmailTemplateFeedbackOOB(model))
			return
		}
		_ = h.Render(w, r, http.StatusConflict, operatorview.EmailTemplateEditor(model))
		return
	}
	h.logEmailTemplateError("restore operator email template", err)
	h.renderInternalError(w, r)
}

func (h *UIHandler) operatorEmailTemplate(w http.ResponseWriter, r *http.Request) (email.EffectiveTemplate, bool) {
	key, ok := h.operatorEmailTemplateKey(w, r)
	if !ok {
		return email.EffectiveTemplate{}, false
	}
	effective, err := h.EmailTemplates.Get(r.Context(), key)
	if err != nil {
		h.logEmailTemplateError("get operator email template", err)
		h.renderInternalError(w, r)
		return email.EffectiveTemplate{}, false
	}
	return effective, true
}

func (h *UIHandler) operatorEmailTemplateKey(w http.ResponseWriter, r *http.Request) (domain.EmailTemplate, bool) {
	key := domain.EmailTemplate(chi.URLParam(r, "templateKey"))
	if err := email.ValidateTemplate(key); err != nil {
		h.renderNotFound(w, r)
		return "", false
	}
	return key, true
}

func (h *UIHandler) parseEmailTemplateForm(w http.ResponseWriter, r *http.Request) (emailTemplateForm, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, maxEmailTemplateFormBytes)
	if err := r.ParseForm(); err != nil {
		h.renderRequestError(w, r, http.StatusBadRequest, "Invalid or oversized email template form.")
		return emailTemplateForm{}, false
	}
	revision, err := strconv.ParseInt(r.Form.Get("revision"), 10, 64)
	if err != nil || revision < 0 {
		h.renderRequestError(w, r, http.StatusBadRequest, "Invalid email template revision.")
		return emailTemplateForm{}, false
	}
	return emailTemplateForm{
		SubjectTemplate: r.Form.Get("subject_template"),
		TextTemplate:    r.Form.Get("text_template"),
		HTMLTemplate:    r.Form.Get("html_template"),
		Revision:        revision,
	}, true
}

func (h *UIHandler) parseEmailTemplateRevision(w http.ResponseWriter, r *http.Request) (int64, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, maxEmailTemplateFormBytes)
	if err := r.ParseForm(); err != nil {
		h.renderRequestError(w, r, http.StatusBadRequest, "Invalid email template form.")
		return 0, false
	}
	revision, err := strconv.ParseInt(r.Form.Get("revision"), 10, 64)
	if err != nil || revision <= 0 {
		h.renderRequestError(w, r, http.StatusBadRequest, "Invalid email template revision.")
		return 0, false
	}
	return revision, true
}

func (h *UIHandler) emailTemplateEditorModel(ctx context.Context, effective email.EffectiveTemplate) (operatorview.EmailTemplateEditorModel, error) {
	model := operatorview.EmailTemplateEditorModel{
		Definition:      effective.Definition,
		Source:          effective.Source,
		SubjectTemplate: effective.Definition.DefaultSubjectTemplate,
		TextTemplate:    effective.Definition.DefaultTextTemplate,
		HTMLTemplate:    effective.Definition.DefaultHTMLTemplate,
	}
	if effective.Override != nil {
		model.Revision = effective.Override.Revision
		model.SubjectTemplate = effective.Override.SubjectTemplate
		model.TextTemplate = effective.Override.TextTemplate
		model.HTMLTemplate = effective.Override.HTMLTemplate
	}
	preview, err := h.EmailTemplates.Preview(email.PreviewTemplateInput{
		Template:        effective.Definition.Key,
		SubjectTemplate: model.SubjectTemplate,
		TextTemplate:    model.TextTemplate,
		HTMLTemplate:    model.HTMLTemplate,
	})
	if err != nil {
		return operatorview.EmailTemplateEditorModel{}, err
	}
	model.Preview = &preview
	if err := h.populateEmailTemplateHistory(ctx, &model); err != nil {
		return operatorview.EmailTemplateEditorModel{}, err
	}
	return model, nil
}

func (h *UIHandler) populateEmailTemplateHistory(ctx context.Context, model *operatorview.EmailTemplateEditorModel) error {
	versions, err := h.EmailTemplates.History(ctx, model.Definition.Key)
	if err != nil {
		return err
	}
	model.Versions = versions
	if model.Source == email.TemplateSourceOverride && len(versions) > 0 {
		model.ActiveVersion = versions[0].Version
	}
	return nil
}

func emailTemplateEditorModelFromForm(effective email.EffectiveTemplate, form emailTemplateForm) operatorview.EmailTemplateEditorModel {
	return operatorview.EmailTemplateEditorModel{
		Definition:      effective.Definition,
		Source:          effective.Source,
		Revision:        form.Revision,
		SubjectTemplate: form.SubjectTemplate,
		TextTemplate:    form.TextTemplate,
		HTMLTemplate:    form.HTMLTemplate,
	}
}

func isEmailTemplateValidationError(err error) bool {
	return errors.Is(err, email.ErrEmptyTemplatePart) ||
		errors.Is(err, email.ErrMalformedTemplate) ||
		errors.Is(err, email.ErrUnknownTemplateVariable) ||
		errors.Is(err, email.ErrMissingTemplatePlaceholder) ||
		errors.Is(err, email.ErrInvalidTemplateSubject)
}

func emailTemplateErrorMessage(err error) string {
	return fmt.Sprintf("%v", err)
}

func setEmailTemplateEditorError(model *operatorview.EmailTemplateEditorModel, err error) {
	model.Error = emailTemplateErrorMessage(err)
	location, ok := email.LocateTemplateError(err)
	if !ok {
		return
	}
	model.Diagnostic = &operatorview.EmailTemplateDiagnostic{
		Part:    location.Part,
		Line:    location.Line,
		Message: model.Error,
	}
}

func (h *UIHandler) redirectEmailTemplate(w http.ResponseWriter, r *http.Request, key domain.EmailTemplate, message string) {
	_ = flash.Set(w, flash.Message{Kind: "success", Message: message})
	http.Redirect(w, r, "/auth/operator/emails/"+string(key), http.StatusSeeOther)
}

func (h *UIHandler) renderEmailTemplateMutationSuccess(
	w http.ResponseWriter,
	r *http.Request,
	component templ.Component,
	message string,
) {
	w.Header().Set("HX-Trigger", emailTemplatePersistedEventName)
	_ = h.Render(
		w,
		r,
		http.StatusOK,
		templ.Join(component, toast.ToastMessage(toast.Success, message)),
	)
}

func (h *UIHandler) logEmailTemplateError(message string, err error) {
	if h.Logger != nil {
		h.Logger.Error(message, "err", err)
	}
}
