package config

import (
	"fmt"

	"github.com/caarlos0/env/v11"
)

type Config struct {
	DatabaseURL        string   `env:"DATABASE_URL,required"`
	OIDCIssuerURL      string   `env:"OIDC_ISSUER_URL"`
	OIDCClientID       string   `env:"OIDC_CLIENT_ID"`
	OIDCClientSecret   string   `env:"OIDC_CLIENT_SECRET"`
	OIDCRedirectURL    string   `env:"OIDC_REDIRECT_URL"`
	Port               string   `env:"PORT"          envDefault:"8080"`
	Env                string   `env:"ENV"           envDefault:"development"`
	AutoMigrate        bool     `env:"AUTO_MIGRATE"  envDefault:"false"`
	DevFakeAuth        bool     `env:"DEV_FAKE_AUTH" envDefault:"false"`
	AllowedEmails      []string `env:"ALLOWED_EMAILS" envSeparator:","`
	InitialAdminEmails []string `env:"INITIAL_ADMIN_EMAILS" envSeparator:","`
	StaticDir          string   `env:"STATIC_DIR"`
	DeletePassword     string   `env:"DELETE_PASSWORD" envDefault:""`
	SessionSecure      bool     `env:"SESSION_SECURE"  envDefault:"true"`

	S3Endpoint  string `env:"S3_ENDPOINT"`
	S3Bucket    string `env:"S3_BUCKET"`
	S3AccessKey string `env:"S3_ACCESS_KEY_ID"`
	S3SecretKey string `env:"S3_SECRET_ACCESS_KEY"`
	S3Region    string `env:"S3_REGION"`
	S3UseSSL    bool   `env:"S3_USE_SSL" envDefault:"true"`

	// AuditExportInterval is a Go duration string (e.g. "10s", "1h", "12h").
	// Empty disables the background exporter entirely — it's opt-in, same
	// as S3 itself.
	AuditExportInterval string `env:"AUDIT_EXPORT_INTERVAL"`
}

// S3Configured reports whether enough S3 settings are present to build a
// client — avatars/audit-export features degrade gracefully (disabled)
// rather than failing startup when it's empty.
func (c *Config) S3Configured() bool {
	return c.S3Endpoint != "" && c.S3Bucket != "" && c.S3AccessKey != "" && c.S3SecretKey != ""
}

func Load() (*Config, error) {
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, err
	}
	if cfg.Env == "production" && cfg.DevFakeAuth {
		return nil, fmt.Errorf("DEV_FAKE_AUTH must not be enabled when ENV=production")
	}
	return cfg, nil
}
