package ctrlsubsonic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.senan.xyz/gonic/db"
)

func TestGuestUserSubsonicFunctionality(t *testing.T) {
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

	// Test 1: Verify a temporary guest user can be created
	t.Run("Create temporary guest user for Subsonic API", func(t *testing.T) {
		tempName := "guest_temp_subsonic_1234"
		guestUser := db.User{
			Name:     tempName,
			Password: "guestpass",
			IsAdmin:  false,
		}
		err := dbc.Create(&guestUser).Error
		require.NoError(t, err)

		// Verify the user was created
		var user db.User
		err = dbc.Where("name = ?", tempName).First(&user).Error
		require.NoError(t, err)
		assert.Equal(t, tempName, user.Name)
		assert.Equal(t, "guestpass", user.Password)
		assert.False(t, user.IsAdmin)
	})

	// Test 2: Verify guest access can be disabled
	t.Run("Disable guest access for Subsonic API", func(t *testing.T) {
		err := dbc.Model(&db.Setting{}).
			Where("key = ?", db.GuestEnabled).
			Update("value", "false").Error
		require.NoError(t, err)

		// Verify guest access is disabled
		guestEnabled, err := dbc.GetSetting(db.GuestEnabled)
		require.NoError(t, err)
		assert.Equal(t, "false", guestEnabled)
	})

	// Test 3: Verify guest credentials can be updated
	t.Run("Update guest credentials for Subsonic API", func(t *testing.T) {
		// Update guest username and password
		err := dbc.Model(&db.Setting{}).
			Where("key = ?", db.GuestUsername).
			Update("value", "new_guest").Error
		require.NoError(t, err)

		err = dbc.Model(&db.Setting{}).
			Where("key = ?", db.GuestPassword).
			Update("value", "new_password").Error
		require.NoError(t, err)

		// Verify changes were saved
		guestUsername, err := dbc.GetSetting(db.GuestUsername)
		require.NoError(t, err)
		assert.Equal(t, "new_guest", guestUsername)

		guestPassword, err := dbc.GetSetting(db.GuestPassword)
		require.NoError(t, err)
		assert.Equal(t, "new_password", guestPassword)
	})
}