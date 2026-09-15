package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/caarlos0/env/v11"
)

type Config struct {
	DatabaseURL      string `env:"DATABASE_URL,required"`
	OIDCIssuerURL    string `env:"OIDC_ISSUER_URL"`
	OIDCClientID     string `env:"OIDC_CLIENT_ID"`
	OIDCClientSecret string `env:"OIDC_CLIENT_SECRET"`
	OIDCRedirectURL  string `env:"OIDC_REDIRECT_URL"`
	Port             string `env:"PORT"          envDefault:"8080"`
	// Env defaults to production: a missing or misspelled ENV must not
	// unlock the development-only escape hatches below.
	Env                string   `env:"ENV"           envDefault:"production"`
	AutoMigrate        bool     `env:"AUTO_MIGRATE"  envDefault:"false"`
	DevFakeAuth        bool     `env:"DEV_FAKE_AUTH" envDefault:"false"`
	AllowedEmails      []string `env:"ALLOWED_EMAILS" envSeparator:","`
	InitialAdminEmails []string `env:"INITIAL_ADMIN_EMAILS" envSeparator:","`
	StaticDir          string   `env:"STATIC_DIR"`
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
	// Allowlist, not denylist: DEV_FAKE_AUTH grants unauthenticated admin
	// (auth.Require injects userID=1 and step-up reauth auto-passes), so
	// anything other than an explicit ENV=development must refuse it.
	if cfg.DevFakeAuth && cfg.Env != "development" {
		return nil, fmt.Errorf("DEV_FAKE_AUTH must not be enabled unless ENV=development (ENV=%q)", cfg.Env)
	}
	// AUDIT_EXPORT_INTERVAL reaches time.NewTicker, which panics on a
	// non-positive duration — and the exporter runs in a bare goroutine that
	// middleware.Recoverer does not cover, so "0" (a plausible way to write
	// "off") would crashloop the pod. Empty is the correct way to disable it.
	if cfg.AuditExportInterval != "" {
		d, err := time.ParseDuration(cfg.AuditExportInterval)
		if err != nil {
			return nil, fmt.Errorf("AUDIT_EXPORT_INTERVAL %q is not a duration: %w", cfg.AuditExportInterval, err)
		}
		if d <= 0 {
			return nil, fmt.Errorf("AUDIT_EXPORT_INTERVAL must be positive (got %q); leave it empty to disable export", cfg.AuditExportInterval)
		}
	}
	return cfg, nil
}

// ValidateOIDC fails when sign-in cannot possibly work. It is deliberately not
// part of Load: the migration Job runs the same binary with only DATABASE_URL
// and ENV mounted, so main calls this only on the path that actually serves
// traffic. Without it a Secret that failed to populate boots a "healthy" pod
// that nobody can log into, surfacing only as a 500 at /auth/login.
func (c *Config) ValidateOIDC() error {
	if c.DevFakeAuth {
		return nil
	}
	if missing := c.missingOIDCSettings(); len(missing) > 0 {
		return fmt.Errorf("OIDC is the only way to sign in but %s %s empty", strings.Join(missing, ", "), plural(len(missing)))
	}
	return nil
}

// missingOIDCSettings names the OIDC settings that are empty. Without this
// check a Secret that failed to populate boots a "healthy" pod that nobody
// can log into — the failure only surfaces as a 500 at /auth/login.
func (c *Config) missingOIDCSettings() []string {
	var missing []string
	for _, s := range []struct {
		name, value string
	}{
		{"OIDC_ISSUER_URL", c.OIDCIssuerURL},
		{"OIDC_CLIENT_ID", c.OIDCClientID},
		{"OIDC_CLIENT_SECRET", c.OIDCClientSecret},
		{"OIDC_REDIRECT_URL", c.OIDCRedirectURL},
	} {
		if s.value == "" {
			missing = append(missing, s.name)
		}
	}
	return missing
}

func plural(n int) string {
	if n == 1 {
		return "is"
	}
	return "are"
}
