package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// Tunnel represents an active tunnel.
type Tunnel struct {
	ID             uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	UserID         uuid.UUID  `gorm:"type:uuid;not null;index;uniqueIndex:idx_user_savedname" json:"user_id"`
	TokenID        uuid.UUID  `gorm:"type:uuid;not null" json:"token_id"`
	DomainID       *uuid.UUID `gorm:"type:uuid" json:"domain_id,omitempty"`
	OrganizationID *uuid.UUID `gorm:"type:uuid;index" json:"organization_id,omitempty"`

	TunnelType string `gorm:"not null" json:"tunnel_type"` // http, https, tcp, tls
	Subdomain  string `gorm:"index" json:"subdomain"`      // Full subdomain: {custom}-{org}
	RemotePort *int   `json:"remote_port,omitempty"`       // For TCP
	LocalAddr  string `gorm:"not null" json:"local_addr"`

	PublicURL string `json:"public_url"`
	ClientID  string `gorm:"uniqueIndex" json:"client_id"`

	// Persistent tunnel fields
	SavedName    *string `gorm:"uniqueIndex:idx_user_savedname" json:"saved_name,omitempty"`
	IsPersistent bool    `gorm:"default:false;index" json:"is_persistent"`

	Status   string         `gorm:"default:'active';index" json:"status"`
	Metadata datatypes.JSON `gorm:"type:json" json:"metadata,omitempty"`

	BytesIn       int64 `gorm:"default:0" json:"bytes_in"`
	BytesOut      int64 `gorm:"default:0" json:"bytes_out"`
	RequestsCount int64 `gorm:"default:0" json:"requests_count"`

	ConnectedAt    time.Time  `json:"connected_at"`
	DisconnectedAt *time.Time `json:"disconnected_at,omitempty"`
	LastActivityAt time.Time  `json:"last_activity_at"`

	// Relationships - omit from JSON to avoid nested data
	User         *User         `gorm:"foreignKey:UserID" json:"-"`
	Token        *AuthToken    `gorm:"foreignKey:TokenID" json:"-"`
	Domain       *Domain       `gorm:"foreignKey:DomainID" json:"-"`
	Organization *Organization `gorm:"foreignKey:OrganizationID" json:"-"`
}

// BeforeCreate hook to set UUID and timestamps.
func (t *Tunnel) BeforeCreate(_ *gorm.DB) error {
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	now := time.Now()
	t.ConnectedAt = now
	t.LastActivityAt = now
	return nil
}

// TableName specifies the table name.
func (Tunnel) TableName() string {
	return "tunnels"
}
