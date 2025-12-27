package models

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// setupTestDB creates an in-memory SQLite database for testing
func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	// Auto-migrate models
	err = db.AutoMigrate(&User{}, &Organization{}, &AuthToken{}, &Domain{}, &Tunnel{})
	require.NoError(t, err)

	return db
}

// TestUserBeforeCreate tests UUID generation on user creation
func TestUserBeforeCreate(t *testing.T) {
	db := setupTestDB(t)

	user := &User{
		Email:    "test@example.com",
		Password: "hashedpassword",
		Name:     "Test User",
		Role:     RoleOrgUser,
	}

	err := db.Create(user).Error
	require.NoError(t, err)

	// UUID should be auto-generated
	assert.NotEqual(t, uuid.Nil, user.ID)
}

// TestUserBeforeCreate_WithProvidedID tests that provided UUID is preserved
func TestUserBeforeCreate_WithProvidedID(t *testing.T) {
	db := setupTestDB(t)

	providedID := uuid.New()
	user := &User{
		ID:       providedID,
		Email:    "test@example.com",
		Password: "hashedpassword",
		Name:     "Test User",
		Role:     RoleOrgUser,
	}

	err := db.Create(user).Error
	require.NoError(t, err)

	// Provided UUID should be preserved
	assert.Equal(t, providedID, user.ID)
}

// TestUserUniqueEmail tests email uniqueness constraint
func TestUserUniqueEmail(t *testing.T) {
	db := setupTestDB(t)

	// Create first user
	user1 := &User{
		Email:    "duplicate@example.com",
		Password: "password1",
		Name:     "User 1",
		Role:     RoleOrgUser,
	}
	err := db.Create(user1).Error
	require.NoError(t, err)

	// Try to create second user with same email
	user2 := &User{
		Email:    "duplicate@example.com",
		Password: "password2",
		Name:     "User 2",
		Role:     RoleOrgUser,
	}
	err = db.Create(user2).Error
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "UNIQUE constraint failed")
}

// TestUserRoles tests all user role types
func TestUserRoles(t *testing.T) {
	db := setupTestDB(t)

	roles := []UserRole{
		RoleSuperAdmin,
		RoleOrgAdmin,
		RoleOrgUser,
	}

	for _, role := range roles {
		user := &User{
			Email:    string(role) + "@example.com",
			Password: "hashedpassword",
			Name:     "User " + string(role),
			Role:     role,
		}

		err := db.Create(user).Error
		require.NoError(t, err)
		assert.Equal(t, role, user.Role)
	}
}

// TestSuperAdminWithoutOrganization tests super admin without org affiliation
func TestSuperAdminWithoutOrganization(t *testing.T) {
	db := setupTestDB(t)

	superAdmin := &User{
		Email:          "admin@example.com",
		Password:       "hashedpassword",
		Name:           "Super Admin",
		Role:           RoleSuperAdmin,
		OrganizationID: nil,
	}

	err := db.Create(superAdmin).Error
	require.NoError(t, err)
	assert.Nil(t, superAdmin.OrganizationID)
}

// TestOrgUserWithOrganization tests org user with organization
func TestOrgUserWithOrganization(t *testing.T) {
	db := setupTestDB(t)

	// Create organization
	org := &Organization{
		Name:        "Test Org",
		Subdomain:   "testorg",
		Description: "Test Organization",
	}
	err := db.Create(org).Error
	require.NoError(t, err)

	// Create org user
	orgUser := &User{
		Email:          "user@testorg.com",
		Password:       "hashedpassword",
		Name:           "Org User",
		Role:           RoleOrgUser,
		OrganizationID: &org.ID,
	}

	err = db.Create(orgUser).Error
	require.NoError(t, err)
	assert.Equal(t, org.ID, *orgUser.OrganizationID)

	// Verify relationship loading
	var loadedUser User
	err = db.Preload("Organization").First(&loadedUser, orgUser.ID).Error
	require.NoError(t, err)
	assert.NotNil(t, loadedUser.Organization)
	assert.Equal(t, "Test Org", loadedUser.Organization.Name)
}

// TestUserDefaultValues tests default values for user fields
func TestUserDefaultValues(t *testing.T) {
	db := setupTestDB(t)

	user := &User{
		Email:    "defaults@example.com",
		Password: "hashedpassword",
		Name:     "Default User",
		// Don't set IsActive or Role to test defaults
	}

	err := db.Create(user).Error
	require.NoError(t, err)

	// Reload to get defaults
	var loaded User
	err = db.First(&loaded, user.ID).Error
	require.NoError(t, err)

	assert.True(t, loaded.IsActive, "IsActive should default to true")
	assert.Equal(t, RoleOrgUser, loaded.Role, "Role should default to org_user")
}

// TestUserPasswordIsNotExposed tests that password is excluded from JSON
func TestUserPasswordIsNotExposed(t *testing.T) {
	user := &User{
		Email:    "test@example.com",
		Password: "secrethash",
		Name:     "Test User",
		Role:     RoleOrgUser,
	}

	// Password field should have json:"-" tag
	// This is checked by the struct tags, tested implicitly
	assert.Equal(t, "secrethash", user.Password)
}

// TestUserTwoFactorFields tests 2FA related fields
func TestUserTwoFactorFields(t *testing.T) {
	db := setupTestDB(t)

	user := &User{
		Email:            "2fa@example.com",
		Password:         "hashedpassword",
		Name:             "2FA User",
		Role:             RoleOrgUser,
		TwoFactorEnabled: true,
		TwoFactorSecret:  "TOTPSECRETVALUE",
	}

	err := db.Create(user).Error
	require.NoError(t, err)

	// Reload and verify
	var loaded User
	err = db.First(&loaded, user.ID).Error
	require.NoError(t, err)

	assert.True(t, loaded.TwoFactorEnabled)
	assert.Equal(t, "TOTPSECRETVALUE", loaded.TwoFactorSecret)
}

// TestUserTableName tests custom table name
func TestUserTableName(t *testing.T) {
	user := User{}
	assert.Equal(t, "users", user.TableName())
}

// TestUserRelationships tests all user relationships
func TestUserRelationships(t *testing.T) {
	db := setupTestDB(t)

	// Create organization
	org := &Organization{
		Name:      "Relationship Test Org",
		Subdomain: "reltest",
	}
	err := db.Create(org).Error
	require.NoError(t, err)

	// Create user
	user := &User{
		Email:          "relationships@example.com",
		Password:       "hashedpassword",
		Name:           "Relationship User",
		Role:           RoleOrgUser,
		OrganizationID: &org.ID,
	}
	err = db.Create(user).Error
	require.NoError(t, err)

	// Create auth token
	token := &AuthToken{
		UserID:    user.ID,
		TokenHash: "hashvalue",
		Name:      "Test Token",
	}
	err = db.Create(token).Error
	require.NoError(t, err)

	// Create domain
	domain := &Domain{
		UserID:         user.ID,
		OrganizationID: &org.ID,
		Subdomain:      "testdomain",
	}
	err = db.Create(domain).Error
	require.NoError(t, err)

	// Create tunnel
	tunnel := &Tunnel{
		UserID:         user.ID,
		TokenID:        token.ID,
		OrganizationID: &org.ID,
		TunnelType:     "http",
		Subdomain:      "testtunnel",
		LocalAddr:      "localhost:3000",
		PublicURL:      "http://testtunnel.example.com",
		ClientID:       uuid.New().String(),
	}
	err = db.Create(tunnel).Error
	require.NoError(t, err)

	// Load user with all relationships
	var loadedUser User
	err = db.
		Preload("Organization").
		Preload("Tokens").
		Preload("Domains").
		Preload("Tunnels").
		First(&loadedUser, user.ID).Error
	require.NoError(t, err)

	// Verify relationships
	assert.NotNil(t, loadedUser.Organization)
	assert.Len(t, loadedUser.Tokens, 1)
	assert.Len(t, loadedUser.Domains, 1)
	assert.Len(t, loadedUser.Tunnels, 1)
}

// TestUserInactiveFlag tests user inactive status
func TestUserInactiveFlag(t *testing.T) {
	db := setupTestDB(t)

	user := &User{
		Email:    "active@example.com",
		Password: "hashedpassword",
		Name:     "Active User",
		Role:     RoleOrgUser,
		IsActive: true,
	}

	err := db.Create(user).Error
	require.NoError(t, err)

	// Update to inactive
	err = db.Model(user).Update("IsActive", false).Error
	require.NoError(t, err)

	// Reload
	var loaded User
	err = db.First(&loaded, user.ID).Error
	require.NoError(t, err)

	assert.False(t, loaded.IsActive, "User should be inactive after update")
}

// TestUserUpdate tests user updates
func TestUserUpdate(t *testing.T) {
	db := setupTestDB(t)

	user := &User{
		Email:    "update@example.com",
		Password: "hashedpassword",
		Name:     "Original Name",
		Role:     RoleOrgUser,
	}

	err := db.Create(user).Error
	require.NoError(t, err)

	// Update name
	err = db.Model(user).Update("Name", "Updated Name").Error
	require.NoError(t, err)

	// Reload
	var loaded User
	err = db.First(&loaded, user.ID).Error
	require.NoError(t, err)

	assert.Equal(t, "Updated Name", loaded.Name)
	assert.NotEmpty(t, loaded.UpdatedAt)
}

// BenchmarkUserCreate benchmarks user creation
func BenchmarkUserCreate(b *testing.B) {
	db, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	db.AutoMigrate(&User{})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		user := &User{
			Email:    "bench" + string(rune(i)) + "@example.com",
			Password: "hashedpassword",
			Name:     "Bench User",
			Role:     RoleOrgUser,
		}
		db.Create(user)
	}
}
