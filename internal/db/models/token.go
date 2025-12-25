package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// AuthToken represents an authentication token
type AuthToken struct {
	ID         uuid.UUID      `gorm:"type:uuid;primaryKey"`
	UserID     uuid.UUID      `gorm:"type:uuid;not null;index"`
	TokenHash  string         `gorm:"uniqueIndex;not null"` // SHA256 hash
	Name       string
	Scopes     datatypes.JSON `gorm:"type:json"`
	LastUsedAt *time.Time
	ExpiresAt  *time.Time
	IsActive   bool      `gorm:"default:true"`
	CreatedAt  time.Time

	// Relationships
	User User `gorm:"foreignKey:UserID"`
}

// BeforeCreate hook to set UUID if not provided
func (t *AuthToken) BeforeCreate(tx *gorm.DB) error {
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	return nil
}

// TableName specifies the table name
func (AuthToken) TableName() string {
	return "auth_tokens"
}
