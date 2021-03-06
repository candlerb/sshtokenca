package util

import (
	"context"
	"fmt"
	oidc "github.com/coreos/go-oidc"
	"golang.org/x/oauth2"
	"regexp"
	"strings"
)

type OpenIDC struct {
	Issuer       string   `yaml:"issuer"`
	ClientID     string   `yaml:"client_id"`
	ClientSecret string   `yaml:"client_secret"`
	RedirectURL  string   `yaml:"redirect_url"`
	Scopes       []string `yaml:"scopes"`

	oauth2           *oauth2.Config
	provider         *oidc.Provider
	verifier         *oidc.IDTokenVerifier
	validRedirectURI *regexp.Regexp
}

// Initialise - makes an outbound connection to fetch the provider
// configuration from the Issuer/.well-known/configuration URL
//
// Note that the ctx is only used for the duration of this call,
// it is not stored anywhere
func (app *OpenIDC) Init(ctx context.Context) error {
	var err error

	if app.Issuer == "" {
		return fmt.Errorf("issuer is missing")
	}
	if app.ClientID == "" {
		return fmt.Errorf("client_id is missing")
	}
	if app.RedirectURL == "" {
		app.RedirectURL = "urn:ietf:wg:oauth:2.0:oob"
	}
	if len(app.Scopes) == 0 {
		app.Scopes = []string{oidc.ScopeOpenID}
	}

	app.validRedirectURI = regexp.MustCompile(`\Ahttp://(localhost|127[.]0[.]0[.]1):\d+/\S*\z`)
	app.provider, err = oidc.NewProvider(ctx, app.Issuer)
	if err != nil {
		return err
	}
	app.verifier = app.provider.Verifier(&oidc.Config{ClientID: app.ClientID})
	// https://godoc.org/golang.org/x/oauth2#Config
	app.oauth2 = &oauth2.Config{
		ClientID:     app.ClientID,
		ClientSecret: app.ClientSecret,
		RedirectURL:  app.RedirectURL,
		Endpoint:     app.provider.Endpoint(),
		Scopes:       app.Scopes,
	}

	return nil
}

func (app *OpenIDC) CodeToIDToken(ctx context.Context, code string) (*oidc.IDToken, error) {
	// Special case: allow user to enter <code><space><URI> so that they can
	// select their own localhost port
	opt := make([]oauth2.AuthCodeOption, 0)
	pieces := strings.Split(code, " ")
	if len(pieces) == 2 && app.validRedirectURI.MatchString(pieces[1]) {
		code = pieces[0]
		opt = append(opt, oauth2.SetAuthURLParam("redirect_uri", pieces[1]))
	}

	// Call out to exchange code for token
	oauth2Token, err := app.oauth2.Exchange(ctx, code, opt...)
	if err != nil {
		return nil, err
	}

	// Extract the ID Token from OAuth2 token.
	rawIDToken, ok := oauth2Token.Extra("id_token").(string)
	if !ok {
		return nil, err
	}

	// Parse and verify ID Token payload.
	idToken, err := app.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, err
	}

	return idToken, nil
}

func (app *OpenIDC) AuthCodeURL(state string) string {
	return app.oauth2.AuthCodeURL(state)
}
