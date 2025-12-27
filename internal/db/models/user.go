package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// UserRole represents the role of a user.
type UserRole string

const (
	RoleSuperAdmin UserRole = "super_admin" // Platform-wide admin, no org affiliation
	RoleOrgAdmin   UserRole = "org_admin"   // Organization administrator
	RoleOrgUser    UserRole = "org_user"    // Regular organization user
)

// User represents a user in the system.
type User struct {
	ID       uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	Email    string    `gorm:"uniqueIndex;not null" json:"email"`
	Password string    `gorm:"not null" json:"-"` // bcrypt hash, never expose in JSON
	Name     string    `json:"name"`
	IsActive bool      `gorm:"default:true" json:"is_active"`

	// Organization fields
	OrganizationID *uuid.UUID `gorm:"type:uuid;index" json:"organization_id,omitempty"` // NULL for super_admins
	Role           UserRole   `gorm:"type:varchar(20);not null;default:'org_user'" json:"role"`

	// 2FA fields
	TwoFactorEnabled bool   `gorm:"default:false" json:"two_factor_enabled"`
	TwoFactorSecret  string `gorm:"type:varchar(255)" json:"-"` // TOTP secret, never expose

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Relationships
	Organization *Organization `gorm:"foreignKey:OrganizationID" json:"-"`
	Tokens       []AuthToken   `gorm:"foreignKey:UserID" json:"-"`
	Domains      []Domain      `gorm:"foreignKey:UserID" json:"-"`
	Tunnels      []Tunnel      `gorm:"foreignKey:UserID" json:"-"`
}

// BeforeCreate hook to set UUID if not provided.
func (u *User) BeforeCreate(_ *gorm.DB) error {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	return nil
}

// TableName specifies the table name.
func (User) TableName() string {
	return "users"
}
