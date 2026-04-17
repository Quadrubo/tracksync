package config

import (
	"fmt"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/spf13/viper"
)

var validate = validator.New(validator.WithRequiredStructEnabled())

func init() {
	validate.RegisterStructValidation(validateConfig, Config{})
}

func validateConfig(sl validator.StructLevel) {
	cfg := sl.Current().Interface().(Config)

	accountDevices := make(map[string]bool)
	for _, a := range cfg.Accounts {
		accountDevices[a.DeviceID] = true
	}

	for i, c := range cfg.Clients {
		for _, deviceID := range c.AllowedDeviceIDs {
			if !accountDevices[deviceID] {
				sl.ReportError(
					c.AllowedDeviceIDs,
					fmt.Sprintf("Clients[%d].AllowedDeviceIDs", i),
					"AllowedDeviceIDs",
					"allowed_device_exists",
					deviceID,
				)
			}
		}
	}
}

type Config struct {
	Port                  string
	StateDB               string
	TargetTimeout         time.Duration
	PassthroughConversion bool
	Accounts              []Account `validate:"required,dive"`
	Clients               []Client  `validate:"required,dive"`
}

type Account struct {
	DeviceID   string `env:"DEVICE_ID" validate:"required"`
	TargetType string `env:"TARGET_TYPE" default:"dawarich" validate:"required"`
	TargetURL  string `env:"TARGET_URL" validate:"required"`
	APIKey     string `env:"API_KEY" validate:"required_without=APIKeyFile"`
	APIKeyFile string `env:"API_KEY_FILE" validate:"required_without=APIKey"`
}

type Client struct {
	ID               string   `env:"ID" validate:"required"`
	Token            string   `env:"TOKEN" validate:"required_without=TokenFile"`
	TokenFile        string   `env:"TOKEN_FILE" validate:"required_without=Token"`
	AllowedDeviceIDs []string `env:"ALLOWED_DEVICES"`
}

func Load(envFile string) (*Config, error) {
	v := viper.New()
	v.SetDefault("PORT", "8080")
	v.SetDefault("STATE_DB", "data/state.db")
	v.SetDefault("TARGET_TIMEOUT", "30s")
	v.AutomaticEnv()

	if envFile != "" {
		v.SetConfigFile(envFile)
		v.SetConfigType("env")
		if err := v.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("config: reading %s: %w", envFile, err)
		}
	}

	targetTimeout, err := time.ParseDuration(v.GetString("TARGET_TIMEOUT"))
	if err != nil {
		return nil, fmt.Errorf("config: invalid TARGET_TIMEOUT: %w", err)
	}

	cfg := &Config{
		Port:                  v.GetString("PORT"),
		StateDB:               v.GetString("STATE_DB"),
		TargetTimeout:         targetTimeout,
		PassthroughConversion: v.GetBool("PASSTHROUGH_CONVERSION"),
		Accounts:              parseGroup[Account](v, "ACCOUNT", "DEVICE_ID"),
		Clients:               parseGroup[Client](v, "CLIENT", "ID"),
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (cfg *Config) validate() error {
	if err := validate.Struct(cfg); err != nil {
		return fmt.Errorf("config: %w", err)
	}
	return nil
}

// parseGroup reads indexed env var groups (e.g. ACCOUNT__0__*, ACCOUNT__1__*)
// into a slice of T. Fields are mapped via `env` struct tags, with optional
// `default` tags. Slice fields ([]string) are split on commas.
// Iteration stops when the sentinel key is empty.
func parseGroup[T any](v *viper.Viper, prefix, sentinel string) []T {
	var items []T
	rt := reflect.TypeOf((*T)(nil)).Elem()

	for i := 0; ; i++ {
		p := fmt.Sprintf("%s__%d__", prefix, i)
		if v.GetString(p+sentinel) == "" {
			break
		}

		item := reflect.New(rt).Elem()
		for j := 0; j < rt.NumField(); j++ {
			f := rt.Field(j)
			key := f.Tag.Get("env")
			if key == "" {
				continue
			}
			val := v.GetString(p + key)
			if val == "" {
				val = f.Tag.Get("default")
			}
			switch f.Type.Kind() {
			case reflect.String:
				item.Field(j).SetString(val)
			case reflect.Slice:
				if val != "" {
					var parts []string
					for _, s := range strings.Split(val, ",") {
						if s = strings.TrimSpace(s); s != "" {
							parts = append(parts, s)
						}
					}
					item.Field(j).Set(reflect.ValueOf(parts))
				}
			}
		}
		items = append(items, item.Interface().(T))
	}
	return items
}

// ResolveToken returns the client's auth token.
// Prefers inline token; falls back to reading token_file from disk.
func (c *Client) ResolveToken() (string, error) {
	if c.Token != "" {
		return c.Token, nil
	}
	data, err := os.ReadFile(c.TokenFile)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func (c *Client) CanUpload(deviceID string) bool {
	for _, id := range c.AllowedDeviceIDs {
		if id == deviceID {
			return true
		}
	}
	return false
}
