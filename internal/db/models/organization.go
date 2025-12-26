package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Organization represents a tenant organization with its own subdomain.
type Organization struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	Name        string    `gorm:"not null" json:"name"`
	Subdomain   string    `gorm:"uniqueIndex;not null" json:"subdomain"` // org identifier in URLs (e.g., "trofeo")
	Description string    `json:"description,omitempty"`
	IsActive    bool      `gorm:"default:true" json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	// Relationships
	Users   []User   `gorm:"foreignKey:OrganizationID" json:"-"`
	Domains []Domain `gorm:"foreignKey:OrganizationID" json:"-"`
	Tunnels []Tunnel `gorm:"foreignKey:OrganizationID" json:"-"`
}

// BeforeCreate hook to set UUID if not provided.
func (o *Organization) BeforeCreate(tx *gorm.DB) error {
	if o.ID == uuid.Nil {
		o.ID = uuid.New()
	}
	return nil
}

// TableName specifies the table name.
func (Organization) TableName() string {
	return "organizations"
}
