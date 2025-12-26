package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// WebhookRoute represents a routing rule from a webhook app to a specific tunnel.
type WebhookRoute struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	WebhookAppID uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:idx_webhook_routes_app_tunnel,priority:1" json:"webhook_app_id"`
	TunnelID     uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:idx_webhook_routes_app_tunnel,priority:2;index" json:"tunnel_id"`

	IsEnabled bool `gorm:"default:true" json:"is_enabled"` // Toggle on/off
	Priority  int  `gorm:"default:100" json:"priority"`    // For response selection (lower = higher priority)

	// Health tracking
	HealthStatus    string    `gorm:"default:'unknown'" json:"health_status"` // "healthy", "unhealthy", "unknown"
	FailureCount    int       `gorm:"default:0" json:"failure_count"`
	LastHealthCheck time.Time `json:"last_health_check,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Relationships
	WebhookApp WebhookApp `gorm:"foreignKey:WebhookAppID;constraint:OnDelete:CASCADE" json:"-"`
	Tunnel     Tunnel     `gorm:"foreignKey:TunnelID;constraint:OnDelete:CASCADE" json:"tunnel,omitempty"`
}

// BeforeCreate sets UUID if not already set.
func (w *WebhookRoute) BeforeCreate(tx *gorm.DB) error {
	if w.ID == uuid.Nil {
		w.ID = uuid.New()
	}
	return nil
}

// TableName specifies the table name for WebhookRoute.
func (WebhookRoute) TableName() string {
	return "webhook_routes"
}
