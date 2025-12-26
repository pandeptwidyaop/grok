package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Domain represents a custom subdomain reservation
type Domain struct {
	ID             uuid.UUID  `gorm:"type:uuid;primaryKey"`
	UserID         uuid.UUID  `gorm:"type:uuid;not null;index"`
	OrganizationID *uuid.UUID `gorm:"type:uuid;index"`
	Subdomain      string     `gorm:"not null;unique"` // Full subdomain must be globally unique
	IsReserved     bool       `gorm:"default:false"`
	CreatedAt      time.Time

	// Relationships
	User         User          `gorm:"foreignKey:UserID"`
	Organization *Organization `gorm:"foreignKey:OrganizationID"`
}

// BeforeCreate hook to set UUID if not provided
func (d *Domain) BeforeCreate(tx *gorm.DB) error {
	if d.ID == uuid.Nil {
		d.ID = uuid.New()
	}
	return nil
}

// TableName specifies the table name
func (Domain) TableName() string {
	return "domains"
}
