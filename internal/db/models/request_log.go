package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// RequestLog represents a logged HTTP request
type RequestLog struct {
	ID         uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	TunnelID   uuid.UUID `gorm:"type:uuid;not null;index:idx_tunnel_created" json:"tunnel_id"`

	Method     string    `json:"method"`
	Path       string    `json:"path"`
	StatusCode int       `json:"status_code"`
	DurationMs int       `json:"duration_ms"`
	BytesIn    int       `json:"bytes_in"`
	BytesOut   int       `json:"bytes_out"`

	ClientIP  string    `json:"client_ip"`
	CreatedAt time.Time `gorm:"index:idx_tunnel_created" json:"created_at"`

	// Relationships
	Tunnel Tunnel `gorm:"foreignKey:TunnelID" json:"-"`
}

// BeforeCreate hook to set UUID
func (r *RequestLog) BeforeCreate(tx *gorm.DB) error {
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	return nil
}

// TableName specifies the table name
func (RequestLog) TableName() string {
	return "request_logs"
}
