package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// WebhookApp represents a webhook application that can have multiple tunnel routes
type WebhookApp struct {
	ID             uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	OrganizationID uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:idx_webhook_apps_org_name,priority:1" json:"organization_id"`
	UserID         uuid.UUID `gorm:"type:uuid;not null;index" json:"user_id"` // Creator

	Name        string `gorm:"not null;uniqueIndex:idx_webhook_apps_org_name,priority:2" json:"name"` // App identifier (e.g., "payment-app")
	Description string `json:"description,omitempty"`
	IsActive    bool   `gorm:"default:true" json:"is_active"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Relationships
	Organization Organization    `gorm:"foreignKey:OrganizationID;constraint:OnDelete:CASCADE" json:"-"`
	User         User            `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"-"`
	Routes       []WebhookRoute  `gorm:"foreignKey:WebhookAppID" json:"routes,omitempty"`
	Events       []WebhookEvent  `gorm:"foreignKey:WebhookAppID" json:"events,omitempty"`
}

// BeforeCreate sets UUID if not already set
func (w *WebhookApp) BeforeCreate(tx *gorm.DB) error {
	if w.ID == uuid.Nil {
		w.ID = uuid.New()
	}
	return nil
}

// TableName specifies the table name for WebhookApp
func (WebhookApp) TableName() string {
	return "webhook_apps"
}
