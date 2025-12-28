package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// RequestLog represents a logged HTTP request.
type RequestLog struct {
	ID       uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	TunnelID uuid.UUID `gorm:"type:uuid;not null;index:idx_request_logs_tunnel_created,priority:1" json:"tunnel_id"`

	Method     string `json:"method"`
	Path       string `json:"path"`
	StatusCode int    `json:"status_code"`
	DurationMs int    `json:"duration_ms"`
	BytesIn    int    `json:"bytes_in"`
	BytesOut   int    `json:"bytes_out"`

	ClientIP string `json:"client_ip"`
	// Composite index (tunnel_id, created_at ASC) for efficient cleanup and pagination
	CreatedAt time.Time `gorm:"index:idx_request_logs_tunnel_created,priority:2,sort:asc" json:"created_at"`

	// Relationships
	Tunnel Tunnel `gorm:"foreignKey:TunnelID" json:"-"`
}

// BeforeCreate hook to set UUID.
func (r *RequestLog) BeforeCreate(_ *gorm.DB) error {
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	return nil
}

// TableName specifies the table name.
func (RequestLog) TableName() string {
	return "request_logs"
}
