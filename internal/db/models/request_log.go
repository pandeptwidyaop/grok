package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// RequestLog represents a logged HTTP request
type RequestLog struct {
	ID         uuid.UUID `gorm:"type:uuid;primaryKey"`
	TunnelID   uuid.UUID `gorm:"type:uuid;not null;index:idx_tunnel_created"`

	Method     string
	Path       string
	StatusCode int
	DurationMs int
	BytesIn    int
	BytesOut   int

	ClientIP  string
	CreatedAt time.Time `gorm:"index:idx_tunnel_created"`

	// Relationships
	Tunnel Tunnel `gorm:"foreignKey:TunnelID"`
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
