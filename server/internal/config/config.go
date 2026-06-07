package config

import (
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/Quadrubo/tracksync/server/internal/converter"
	"github.com/Quadrubo/tracksync/server/internal/target"
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
	for i, a := range cfg.Accounts {
		accountDevices[a.DeviceID] = true

		if _, err := converter.ParseMarkerRules(a.Markers); err != nil {
			sl.ReportError(
				a.Markers,
				fmt.Sprintf("Accounts[%d].Markers", i),
				"Markers",
				"valid_marker_rules",
				err.Error(),
			)
		}
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
	MaxUploadSize         int64
	PassthroughConversion bool
	Accounts              []Account `validate:"required,dive"`
	Clients               []Client  `validate:"required,dive"`
	// TargetConfigs holds each target type's own config, keyed by type name.
	TargetConfigs map[string]any
}

type Account struct {
	DeviceID            string   `env:"DEVICE_ID" validate:"required"`
	TargetType          string   `env:"TARGET_TYPE" default:"dawarich" validate:"required"`
	TargetURL           string   `env:"TARGET_URL" validate:"required"`
	APIKey              string   `env:"API_KEY" validate:"required_without=APIKeyFile"`
	APIKeyFile          string   `env:"API_KEY_FILE" validate:"required_without=APIKey"`
	Markers             []string `env:"MARKERS"`
	SplitMarkerPosition string   `env:"SPLIT_MARKER_POSITION" default:"start" validate:"omitempty,oneof=start end"`
	SplitMode           string   `env:"SPLIT_MODE" default:"tracks" validate:"omitempty,oneof=tracks files"`
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
	v.SetDefault("MAX_UPLOAD_SIZE", "32")
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

	maxUploadSize := v.GetInt64("MAX_UPLOAD_SIZE")
	if maxUploadSize <= 0 {
		return nil, fmt.Errorf("config: MAX_UPLOAD_SIZE must be positive")
	}

	accounts, err := parseGroup[Account](v, "ACCOUNT", "DEVICE_ID")
	if err != nil {
		return nil, err
	}
	clients, err := parseGroup[Client](v, "CLIENT", "ID")
	if err != nil {
		return nil, err
	}
	targetConfigs, err := parseTargetConfigs(v)
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		Port:                  v.GetString("PORT"),
		StateDB:               v.GetString("STATE_DB"),
		TargetTimeout:         targetTimeout,
		MaxUploadSize:         maxUploadSize << 20, // MB to bytes
		PassthroughConversion: v.GetBool("PASSTHROUGH_CONVERSION"),
		Accounts:              accounts,
		Clients:               clients,
		TargetConfigs:         targetConfigs,
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	for i := range cfg.Clients {
		c := &cfg.Clients[i]
		t, err := c.ResolveToken()
		if err != nil {
			return nil, fmt.Errorf("config: reading token for client %q: %w", c.ID, err)
		}
		c.Token = t
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
// into a slice of T, stopping at the first index whose sentinel key is empty.
func parseGroup[T any](v *viper.Viper, prefix, sentinel string) ([]T, error) {
	var items []T
	rt := reflect.TypeOf((*T)(nil)).Elem()

	for i := 0; ; i++ {
		p := fmt.Sprintf("%s__%d__", prefix, i)
		if v.GetString(p+sentinel) == "" {
			break
		}
		item := reflect.New(rt).Elem()
		if err := fillStruct(v, p, item); err != nil {
			return nil, err
		}
		items = append(items, item.Interface().(T))
	}
	return items, nil
}

// parseTargetConfigs fills each registered target type's config from its
// TARGET__<TYPE>__* env vars, keyed by target type.
func parseTargetConfigs(v *viper.Viper) (map[string]any, error) {
	configs := map[string]any{}
	for typeName, prototype := range target.ConfigPrototypes() {
		prefix := "TARGET__" + strings.ToUpper(typeName) + "__"
		c, err := parseTargetConfig(v, prefix, prototype)
		if err != nil {
			return nil, err
		}
		configs[typeName] = c
	}
	return configs, nil
}

// parseTargetConfig fills a fresh copy of prototype from env vars under prefix.
func parseTargetConfig(v *viper.Viper, prefix string, prototype any) (any, error) {
	rv := reflect.New(reflect.TypeOf(prototype)).Elem()
	if err := fillStruct(v, prefix, rv); err != nil {
		return nil, err
	}
	return rv.Interface(), nil
}

// fillStruct populates struct rv from env vars under prefix, mapping fields via
// `env` tags with optional `default` tags. Fields without an `env` tag are
// skipped; []string fields are split on commas.
func fillStruct(v *viper.Viper, prefix string, rv reflect.Value) error {
	rt := rv.Type()
	for j := 0; j < rt.NumField(); j++ {
		f := rt.Field(j)
		key := f.Tag.Get("env")
		if key == "" {
			continue
		}
		val := v.GetString(prefix + key)
		if val == "" {
			val = f.Tag.Get("default")
		}
		if err := setField(rv.Field(j), prefix+key, val); err != nil {
			return err
		}
	}
	return nil
}

// setField assigns val to a field by kind; slices split on commas. name is the
// env key, used for error messages. An unsupported field kind means a
// misdeclared config struct, so it panics rather than returning an error.
func setField(field reflect.Value, name, val string) error {
	switch field.Kind() {
	case reflect.String:
		field.SetString(val)
	case reflect.Bool:
		if val == "" {
			return nil
		}
		b, err := strconv.ParseBool(strings.TrimSpace(val))
		if err != nil {
			return fmt.Errorf("config: %s: invalid boolean value %q", name, val)
		}
		field.SetBool(b)
	case reflect.Slice:
		if val != "" {
			var parts []string
			for _, s := range strings.Split(val, ",") {
				if s = strings.TrimSpace(s); s != "" {
					parts = append(parts, s)
				}
			}
			field.Set(reflect.ValueOf(parts))
		}
	default:
		panic(fmt.Sprintf("config: %s: unsupported field kind %s", name, field.Kind()))
	}
	return nil
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
