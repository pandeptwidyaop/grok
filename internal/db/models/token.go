package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// AuthToken represents an authentication token.
type AuthToken struct {
	ID         uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	UserID     uuid.UUID      `gorm:"type:uuid;not null;index" json:"user_id"`
	TokenHash  string         `gorm:"uniqueIndex;not null" json:"-"` // SHA256 hash - never expose
	Name       string         `json:"name"`
	Scopes     datatypes.JSON `gorm:"type:json" json:"scopes"`
	LastUsedAt *time.Time     `json:"last_used_at,omitempty"`
	ExpiresAt  *time.Time     `json:"expires_at,omitempty"`
	IsActive   bool           `gorm:"default:true" json:"is_active"`
	CreatedAt  time.Time      `json:"created_at"`

	// Relationships - omit from JSON
	User User `gorm:"foreignKey:UserID" json:"-"`
}

// BeforeCreate hook to set UUID if not provided.
func (t *AuthToken) BeforeCreate(tx *gorm.DB) error {
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	return nil
}

// TableName specifies the table name.
func (AuthToken) TableName() string {
	return "auth_tokens"
}
