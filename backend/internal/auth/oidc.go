package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

type Provider struct {
	provider *oidc.Provider
	config   oauth2.Config
	verifier *oidc.IDTokenVerifier
}

func NewProvider(ctx context.Context, issuerURL, clientID, clientSecret, redirectURL string) (*Provider, error) {
	p, err := oidc.NewProvider(ctx, issuerURL)
	if err != nil {
		return nil, fmt.Errorf("oidc provider: %w", err)
	}
	cfg := oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Endpoint:     p.Endpoint(),
		Scopes:       []string{oidc.ScopeOpenID, "profile", "email"},
	}
	return &Provider{
		provider: p,
		config:   cfg,
		verifier: p.Verifier(&oidc.Config{ClientID: clientID}),
	}, nil
}

func (p *Provider) AuthCodeURL(state, nonce string) string {
	return p.config.AuthCodeURL(state, oidc.Nonce(nonce))
}

func (p *Provider) Exchange(ctx context.Context, code string) (*oidc.IDToken, error) {
	token, err := p.config.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("exchange: %w", err)
	}
	rawID, ok := token.Extra("id_token").(string)
	if !ok {
		return nil, fmt.Errorf("no id_token")
	}
	idToken, err := p.verifier.Verify(ctx, rawID)
	if err != nil {
		return nil, fmt.Errorf("verify: %w", err)
	}
	return idToken, nil
}

func RandomString(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b)[:n], nil
}
