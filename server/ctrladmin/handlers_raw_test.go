package ctrladmin

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.senan.xyz/gonic/db"
)

func TestGuestUserFunctionality(t *testing.T) {
	// Create a mock database
	dbc, err := db.NewMock()
	require.NoError(t, err)

	err = dbc.AutoMigrate(
		db.User{},
		db.Setting{},
	).Error
	require.NoError(t, err)

	// Create admin user
	err = dbc.Create(&db.User{
		Name:     "admin",
		Password: "admin",
		IsAdmin:  true,
	}).Error
	require.NoError(t, err)

	// Set up guest settings
	err = dbc.Create(&db.Setting{
		Key:   db.GuestEnabled,
		Value: "true",
	}).Error
	require.NoError(t, err)

	err = dbc.Create(&db.Setting{
		Key:   db.GuestUsername,
		Value: "guest",
	}).Error
	require.NoError(t, err)

	err = dbc.Create(&db.Setting{
		Key:   db.GuestPassword,
		Value: "guestpass",
	}).Error
	require.NoError(t, err)

	// Test 1: Verify guest credentials match settings
	t.Run("Guest credentials match settings", func(t *testing.T) {
		guestEnabled, err := dbc.GetSetting(db.GuestEnabled)
		require.NoError(t, err)
		assert.Equal(t, "true", guestEnabled)

		guestUsername, err := dbc.GetSetting(db.GuestUsername)
		require.NoError(t, err)
		assert.Equal(t, "guest", guestUsername)

		guestPassword, err := dbc.GetSetting(db.GuestPassword)
		require.NoError(t, err)
		assert.Equal(t, "guestpass", guestPassword)
	})

	// Test 2: Create a temporary guest user
	t.Run("Create temporary guest user", func(t *testing.T) {
		// Create a temporary guest user
		guestUser := db.User{
			Name:     "guest_temp_123456",
			Password: "guestpass",
			IsAdmin:  false,
		}
		err := dbc.Create(&guestUser).Error
		require.NoError(t, err)

		// Verify the user was created
		var count int
		err = dbc.Model(&db.User{}).Where("name = ?", "guest_temp_123456").Count(&count).Error
		require.NoError(t, err)
		assert.Equal(t, 1, count)

		// Verify the user is not an admin
		var user db.User
		err = dbc.Where("name = ?", "guest_temp_123456").First(&user).Error
		require.NoError(t, err)
		assert.False(t, user.IsAdmin)
	})

	// Test 3: Disable guest access
	t.Run("Disable guest access", func(t *testing.T) {
		// Disable guest access
		err := dbc.Model(&db.Setting{}).
			Where("key = ?", db.GuestEnabled).
			Update("value", "false").Error
		require.NoError(t, err)

		// Verify guest access is disabled
		guestEnabled, err := dbc.GetSetting(db.GuestEnabled)
		require.NoError(t, err)
		assert.Equal(t, "false", guestEnabled)
	})
}