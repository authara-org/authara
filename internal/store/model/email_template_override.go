package model

import (
	"time"

	"github.com/google/uuid"
)

type EmailTemplateOverride struct {
	TemplateKey string `db:"template_key"`

	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`

	SubjectTemplate string     `db:"subject_template"`
	TextTemplate    string     `db:"text_template"`
	HTMLTemplate    string     `db:"html_template"`
	Revision        int64      `db:"revision"`
	UpdatedByUserID *uuid.UUID `db:"updated_by_user_id"`
}

func (EmailTemplateOverride) TableName() string {
	return "email_template_overrides"
}
