package model

import (
	"time"

	"github.com/google/uuid"
)

type EmailTemplateDeliverySetting struct {
	TemplateKey string `db:"template_key"`
	Enabled     bool   `db:"enabled"`

	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`

	UpdatedByUserID *uuid.UUID `db:"updated_by_user_id"`
}

func (EmailTemplateDeliverySetting) TableName() string {
	return "email_template_delivery_settings"
}
