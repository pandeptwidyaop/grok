package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// Tunnel represents an active tunnel
type Tunnel struct {
	ID         uuid.UUID      `gorm:"type:uuid;primaryKey"`
	UserID     uuid.UUID      `gorm:"type:uuid;not null;index"`
	TokenID    uuid.UUID      `gorm:"type:uuid;not null"`
	DomainID   *uuid.UUID     `gorm:"type:uuid"`

	TunnelType string `gorm:"not null"` // http, https, tcp, tls
	Subdomain  string `gorm:"index"`
	RemotePort *int   // For TCP
	LocalAddr  string `gorm:"not null"`

	PublicURL string
	ClientID  string `gorm:"uniqueIndex"`

	Status   string         `gorm:"default:'active';index"`
	Metadata datatypes.JSON `gorm:"type:json"`

	BytesIn       int64 `gorm:"default:0"`
	BytesOut      int64 `gorm:"default:0"`
	RequestsCount int64 `gorm:"default:0"`

	ConnectedAt    time.Time
	DisconnectedAt *time.Time
	LastActivityAt time.Time

	// Relationships
	User   User       `gorm:"foreignKey:UserID"`
	Token  AuthToken  `gorm:"foreignKey:TokenID"`
	Domain *Domain    `gorm:"foreignKey:DomainID"`
}

// BeforeCreate hook to set UUID and timestamps
func (t *Tunnel) BeforeCreate(tx *gorm.DB) error {
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	now := time.Now()
	t.ConnectedAt = now
	t.LastActivityAt = now
	return nil
}

// TableName specifies the table name
func (Tunnel) TableName() string {
	return "tunnels"
}
