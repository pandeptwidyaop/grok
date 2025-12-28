package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// WebhookEvent represents a logged webhook request event.
type WebhookEvent struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	WebhookAppID uuid.UUID `gorm:"type:uuid;not null;index:idx_webhook_events_app_created,priority:1" json:"webhook_app_id"`

	RequestPath string `gorm:"not null" json:"request_path"` // Full path: /app_name/stripe/callback
	Method      string `gorm:"not null" json:"method"`       // HTTP method
	StatusCode  int    `json:"status_code"`                  // Response status code
	DurationMs  int64  `json:"duration_ms"`                  // Request duration in milliseconds
	BytesIn     int64  `gorm:"default:0" json:"bytes_in"`    // Request body size
	BytesOut    int64  `gorm:"default:0" json:"bytes_out"`   // Response body size
	ClientIP    string `json:"client_ip,omitempty"`          // Client IP address

	RoutingStatus string `json:"routing_status,omitempty"`       // "success", "partial", "failed"
	TunnelCount   int    `gorm:"default:0" json:"tunnel_count"`  // Number of tunnels that received the request
	SuccessCount  int    `gorm:"default:0" json:"success_count"` // Number of successful responses
	ErrorMessage  string `json:"error_message,omitempty"`

	// Extended fields for detailed request/response capture
	RequestHeaders  string `gorm:"type:text" json:"request_headers,omitempty"`  // JSON-encoded headers
	RequestBody     string `gorm:"type:text" json:"request_body,omitempty"`     // Request body (may be truncated)
	ResponseHeaders string `gorm:"type:text" json:"response_headers,omitempty"` // From first successful response
	ResponseBody    string `gorm:"type:text" json:"response_body,omitempty"`    // From first successful response
	BodyTruncated   bool   `gorm:"default:false" json:"body_truncated"`         // Indicates truncation

	// Composite index (webhook_app_id, created_at DESC) for efficient event queries
	CreatedAt time.Time `gorm:"index:idx_webhook_events_app_created,priority:2,sort:desc" json:"created_at"`

	// Relationships
	WebhookApp WebhookApp `gorm:"foreignKey:WebhookAppID;constraint:OnDelete:CASCADE" json:"-"`
}

// BeforeCreate sets UUID if not already set.
func (w *WebhookEvent) BeforeCreate(_ *gorm.DB) error {
	if w.ID == uuid.Nil {
		w.ID = uuid.New()
	}
	return nil
}

// TableName specifies the table name for WebhookEvent.
func (WebhookEvent) TableName() string {
	return "webhook_events"
}
