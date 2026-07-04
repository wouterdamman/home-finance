package config

import (
	"github.com/caarlos0/env/v11"
)

type Config struct {
	DatabaseURL      string   `env:"DATABASE_URL,required"`
	OIDCIssuerURL    string   `env:"OIDC_ISSUER_URL"`
	OIDCClientID     string   `env:"OIDC_CLIENT_ID"`
	OIDCClientSecret string   `env:"OIDC_CLIENT_SECRET"`
	OIDCRedirectURL  string   `env:"OIDC_REDIRECT_URL"`
	SessionKey       string   `env:"SESSION_KEY,required"`
	Port             string   `env:"PORT"          envDefault:"8080"`
	Env              string   `env:"ENV"           envDefault:"development"`
	AutoMigrate      bool     `env:"AUTO_MIGRATE"  envDefault:"false"`
	DevFakeAuth      bool     `env:"DEV_FAKE_AUTH" envDefault:"false"`
	AllowedEmails    []string `env:"ALLOWED_EMAILS" envSeparator:","`
	StaticDir        string   `env:"STATIC_DIR"`
}

func Load() (*Config, error) {
	cfg := &Config{}
	return cfg, env.Parse(cfg)
}
