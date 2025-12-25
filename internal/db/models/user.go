package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// User represents a user in the system
type User struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	Email     string    `gorm:"uniqueIndex;not null"`
	Password  string    `gorm:"not null"` // bcrypt hash
	Name      string
	IsActive  bool      `gorm:"default:true"`
	CreatedAt time.Time
	UpdatedAt time.Time

	// Relationships
	Tokens  []AuthToken `gorm:"foreignKey:UserID"`
	Domains []Domain    `gorm:"foreignKey:UserID"`
	Tunnels []Tunnel    `gorm:"foreignKey:UserID"`
}

// BeforeCreate hook to set UUID if not provided
func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	return nil
}

// TableName specifies the table name
func (User) TableName() string {
	return "users"
}
