package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseGroup_Accounts(t *testing.T) {
	v := viper.New()
	v.Set("ACCOUNT__0__DEVICE_ID", "dev-1")
	v.Set("ACCOUNT__0__TARGET_TYPE", "dawarich")
	v.Set("ACCOUNT__0__TARGET_URL", "http://localhost:3000")
	v.Set("ACCOUNT__0__API_KEY", "key-1")
	v.Set("ACCOUNT__1__DEVICE_ID", "dev-2")
	v.Set("ACCOUNT__1__TARGET_URL", "http://localhost:3001")
	v.Set("ACCOUNT__1__API_KEY_FILE", "/tmp/key")

	accounts := parseGroup[Account](v, "ACCOUNT", "DEVICE_ID")

	require.Len(t, accounts, 2)
	assert.Equal(t, "dev-1", accounts[0].DeviceID)
	assert.Equal(t, "dawarich", accounts[0].TargetType)
	assert.Equal(t, "key-1", accounts[0].APIKey)
	assert.Equal(t, "/tmp/key", accounts[1].APIKeyFile)
}

func TestParseGroup_DefaultTargetType(t *testing.T) {
	v := viper.New()
	v.Set("ACCOUNT__0__DEVICE_ID", "dev-1")
	v.Set("ACCOUNT__0__TARGET_URL", "http://localhost:3000")
	v.Set("ACCOUNT__0__API_KEY", "key")

	accounts := parseGroup[Account](v, "ACCOUNT", "DEVICE_ID")

	require.Len(t, accounts, 1)
	assert.Equal(t, "dawarich", accounts[0].TargetType)
}

func TestParseGroup_Clients(t *testing.T) {
	v := viper.New()
	v.Set("CLIENT__0__ID", "laptop")
	v.Set("CLIENT__0__TOKEN", "tok")
	v.Set("CLIENT__0__ALLOWED_DEVICES", "dev-1, dev-2 , dev-3")

	clients := parseGroup[Client](v, "CLIENT", "ID")

	require.Len(t, clients, 1)
	assert.Equal(t, "laptop", clients[0].ID)
	assert.Equal(t, []string{"dev-1", "dev-2", "dev-3"}, clients[0].AllowedDeviceIDs)
}

func TestParseGroup_StopsAtGap(t *testing.T) {
	v := viper.New()
	v.Set("ACCOUNT__0__DEVICE_ID", "dev-0")
	v.Set("ACCOUNT__0__TARGET_URL", "http://localhost")
	v.Set("ACCOUNT__0__API_KEY", "key")
	// skip index 1
	v.Set("ACCOUNT__2__DEVICE_ID", "dev-2")
	v.Set("ACCOUNT__2__TARGET_URL", "http://localhost")
	v.Set("ACCOUNT__2__API_KEY", "key")

	accounts := parseGroup[Account](v, "ACCOUNT", "DEVICE_ID")
	assert.Len(t, accounts, 1, "should stop at gap in indices")
}

func TestValidate_MissingAccounts(t *testing.T) {
	cfg := &Config{Accounts: nil, Clients: []Client{{ID: "c", Token: "t"}}}
	assert.Error(t, cfg.validate())
}

func TestValidate_MissingTargetURL(t *testing.T) {
	cfg := &Config{
		Accounts: []Account{{DeviceID: "d", TargetType: "dawarich", APIKey: "k"}},
		Clients:  []Client{{ID: "c", Token: "t"}},
	}
	assert.Error(t, cfg.validate())
}

func TestValidate_APIKeyOrFile(t *testing.T) {
	cfg := &Config{
		Accounts: []Account{{DeviceID: "d", TargetType: "dawarich", TargetURL: "http://x"}},
		Clients:  []Client{{ID: "c", Token: "t"}},
	}
	assert.Error(t, cfg.validate(), "neither APIKey nor APIKeyFile")

	cfg.Accounts[0].APIKeyFile = "/tmp/key"
	assert.NoError(t, cfg.validate(), "APIKeyFile alone should suffice")
}

func TestValidate_TokenOrFile(t *testing.T) {
	cfg := &Config{
		Accounts: []Account{{DeviceID: "d", TargetType: "dawarich", TargetURL: "http://x", APIKey: "k"}},
		Clients:  []Client{{ID: "c"}},
	}
	assert.Error(t, cfg.validate(), "neither Token nor TokenFile")

	cfg.Clients[0].TokenFile = "/tmp/token"
	assert.NoError(t, cfg.validate(), "TokenFile alone should suffice")
}

func TestValidate_AllowedDevicesUnknown(t *testing.T) {
	cfg := &Config{
		Accounts: []Account{{DeviceID: "dev-1", TargetType: "dawarich", TargetURL: "http://x", APIKey: "k"}},
		Clients:  []Client{{ID: "c", Token: "t", AllowedDeviceIDs: []string{"dev-1", "nonexistent"}}},
	}
	assert.Error(t, cfg.validate())
}

func TestValidate_AllowedDevicesValid(t *testing.T) {
	cfg := &Config{
		Accounts: []Account{{DeviceID: "dev-1", TargetType: "dawarich", TargetURL: "http://x", APIKey: "k"}},
		Clients:  []Client{{ID: "c", Token: "t", AllowedDeviceIDs: []string{"dev-1"}}},
	}
	assert.NoError(t, cfg.validate())
}

func TestResolveToken_Inline(t *testing.T) {
	c := &Client{Token: "inline-token"}
	tok, err := c.ResolveToken()
	require.NoError(t, err)
	assert.Equal(t, "inline-token", tok)
}

func TestResolveToken_File(t *testing.T) {
	path := filepath.Join(t.TempDir(), "token")
	require.NoError(t, os.WriteFile(path, []byte("  file-token\n"), 0600))

	c := &Client{TokenFile: path}
	tok, err := c.ResolveToken()
	require.NoError(t, err)
	assert.Equal(t, "file-token", tok)
}

func TestResolveToken_InlinePreferred(t *testing.T) {
	path := filepath.Join(t.TempDir(), "token")
	require.NoError(t, os.WriteFile(path, []byte("file-token"), 0600))

	c := &Client{Token: "inline", TokenFile: path}
	tok, err := c.ResolveToken()
	require.NoError(t, err)
	assert.Equal(t, "inline", tok, "inline token should take precedence")
}

func TestCanUpload(t *testing.T) {
	c := &Client{AllowedDeviceIDs: []string{"dev-1", "dev-2"}}
	assert.True(t, c.CanUpload("dev-1"))
	assert.True(t, c.CanUpload("dev-2"))
	assert.False(t, c.CanUpload("dev-3"))
}

func TestCanUpload_Empty(t *testing.T) {
	c := &Client{}
	assert.False(t, c.CanUpload("anything"))
}
