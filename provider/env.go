package provider

import "os"

// EffectiveClientID returns the OAuth client ID, resolving from the
// environment variable named by EnvClientID when set. This keeps OAuth
// client credentials out of source-controlled code.
func (d Definition) EffectiveClientID() string {
	if d.EnvClientID != "" {
		if v := os.Getenv(d.EnvClientID); v != "" {
			return v
		}
	}
	return d.ClientID
}

// EffectiveClientSecret returns the OAuth client secret, resolving from
// the environment variable named by EnvClientSecret when set.
func (d Definition) EffectiveClientSecret() string {
	if d.EnvClientSecret != "" {
		if v := os.Getenv(d.EnvClientSecret); v != "" {
			return v
		}
	}
	return d.ClientSecret
}

// NeedsBaseURL reports whether logging in to this provider must also collect
// an endpoint: it has no default base URL and nib cannot derive one (Azure,
// where each account has its own resource URL). OpenAI's empty BaseURL means
// the SDK default, so it does not count.
func (d Definition) NeedsBaseURL() bool {
	return d.LoginKind == LoginAPIKey && d.BaseURL == "" && d.ID != "openai"
}
