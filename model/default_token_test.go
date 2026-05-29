package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting"
	"github.com/stretchr/testify/require"
)

func TestEnsureUserDefaultTokenCreatesOrdinaryUnlimitedToken(t *testing.T) {
	truncateTables(t)
	setting.DefaultUseAutoGroup = false

	token, err := EnsureUserDefaultToken(42, "alice")
	require.NoError(t, err)
	require.NotNil(t, token)

	require.Equal(t, 42, token.UserId)
	require.Equal(t, "Default API Key", token.Name)
	require.Equal(t, TokenPurposeDefault, token.Purpose)
	require.Equal(t, common.TokenStatusEnabled, token.Status)
	require.Equal(t, int64(-1), token.ExpiredTime)
	require.True(t, token.UnlimitedQuota)
	require.False(t, token.ModelLimitsEnabled)
	require.Empty(t, token.ModelLimits)
	require.Empty(t, token.Group)
	require.NotEmpty(t, token.Key)
}

func TestEnsureUserDefaultTokenReusesExistingDefaultToken(t *testing.T) {
	truncateTables(t)

	existing := &Token{
		UserId:             42,
		Name:               "Existing",
		Key:                "existing-default-key",
		Status:             common.TokenStatusEnabled,
		CreatedTime:        1,
		AccessedTime:       1,
		ExpiredTime:        -1,
		UnlimitedQuota:     true,
		ModelLimitsEnabled: false,
		Purpose:            TokenPurposeDefault,
	}
	require.NoError(t, DB.Create(existing).Error)

	token, err := EnsureUserDefaultToken(42, "alice")
	require.NoError(t, err)
	require.Equal(t, existing.Id, token.Id)
	require.Equal(t, "existing-default-key", token.Key)

	var count int64
	require.NoError(t, DB.Model(&Token{}).Where("user_id = ?", 42).Count(&count).Error)
	require.Equal(t, int64(1), count)
}

func TestEnsureUserDefaultTokenDoesNotReplaceDisabledDefaultToken(t *testing.T) {
	truncateTables(t)

	existing := &Token{
		UserId:             42,
		Name:               "Disabled",
		Key:                "disabled-default-key",
		Status:             common.TokenStatusDisabled,
		CreatedTime:        1,
		AccessedTime:       1,
		ExpiredTime:        -1,
		UnlimitedQuota:     true,
		ModelLimitsEnabled: false,
		Purpose:            TokenPurposeDefault,
	}
	require.NoError(t, DB.Create(existing).Error)

	token, err := EnsureUserDefaultToken(42, "alice")
	require.NoError(t, err)
	require.Equal(t, existing.Id, token.Id)
	require.Equal(t, common.TokenStatusDisabled, token.Status)

	var count int64
	require.NoError(t, DB.Model(&Token{}).Where("user_id = ?", 42).Count(&count).Error)
	require.Equal(t, int64(1), count)
}

func TestEnsureUserDefaultTokenUsesAutoGroupWhenConfigured(t *testing.T) {
	truncateTables(t)
	previous := setting.DefaultUseAutoGroup
	setting.DefaultUseAutoGroup = true
	t.Cleanup(func() {
		setting.DefaultUseAutoGroup = previous
	})

	token, err := EnsureUserDefaultToken(42, "alice")
	require.NoError(t, err)
	require.Equal(t, "auto", token.Group)
}

func TestUserInsertCreatesDefaultToken(t *testing.T) {
	truncateTables(t)
	setting.DefaultUseAutoGroup = false

	user := &User{
		Username:    "alice",
		Password:    "password",
		DisplayName: "alice",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
	}
	require.NoError(t, user.Insert(0))

	var token Token
	require.NoError(t, DB.Where("user_id = ? AND purpose = ?", user.Id, TokenPurposeDefault).First(&token).Error)
	require.Equal(t, "Default API Key", token.Name)
	require.True(t, token.UnlimitedQuota)
}

func TestFinalizeOAuthUserCreationCreatesDefaultToken(t *testing.T) {
	truncateTables(t)

	user := &User{
		Username:    "oauth-user",
		DisplayName: "OAuth User",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
	}
	require.NoError(t, DB.Create(user).Error)

	user.FinalizeOAuthUserCreation(0)

	var token Token
	require.NoError(t, DB.Where("user_id = ? AND purpose = ?", user.Id, TokenPurposeDefault).First(&token).Error)
	require.Equal(t, "Default API Key", token.Name)
}
