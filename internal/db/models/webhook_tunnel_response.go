package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// WebhookTunnelResponse represents response from individual tunnel in broadcast.
type WebhookTunnelResponse struct {
	ID             uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	WebhookEventID uuid.UUID `gorm:"type:uuid;not null;index" json:"webhook_event_id"`
	TunnelID       uuid.UUID `gorm:"type:uuid;not null" json:"tunnel_id"`

	TunnelSubdomain string `gorm:"not null" json:"tunnel_subdomain"` // Snapshot at time of request
	StatusCode      int    `json:"status_code"`                      // 0 if failed
	DurationMs      int64  `json:"duration_ms"`                      // Latency for this tunnel
	Success         bool   `gorm:"default:false" json:"success"`
	ErrorMessage    string `json:"error_message,omitempty"`

	ResponseHeaders string `gorm:"type:text" json:"response_headers,omitempty"` // JSON-encoded
	ResponseBody    string `gorm:"type:text" json:"response_body,omitempty"`    // May be truncated

	CreatedAt time.Time `json:"created_at"`

	// Relationships
	WebhookEvent WebhookEvent `gorm:"foreignKey:WebhookEventID;constraint:OnDelete:CASCADE" json:"-"`
}

// BeforeCreate sets UUID if not already set.
func (w *WebhookTunnelResponse) BeforeCreate(_ *gorm.DB) error {
	if w.ID == uuid.Nil {
		w.ID = uuid.New()
	}
	return nil
}

// TableName specifies the table name for WebhookTunnelResponse.
func (WebhookTunnelResponse) TableName() string {
	return "webhook_tunnel_responses"
}
