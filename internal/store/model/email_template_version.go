package model

import (
	"time"

	"github.com/google/uuid"
)

type EmailTemplateVersion struct {
	TemplateKey string `db:"template_key"`
	Version     int64  `db:"version"`
	CreatedAt   time.Time

	SubjectTemplate string
	TextTemplate    string
	HTMLTemplate    string
	CreatedByUserID *uuid.UUID
}

func (EmailTemplateVersion) TableName() string {
	return "email_template_versions"
}
