/*
 * Gerege SSO
 * Copyright (c) 2026 Gerege Systems Development Team, @craftzbay, Gemini AI & Claude AI
 * Distributed under the Apache 2.0 License.
 */

package ssoprovider

import (
	"errors"
	"net/url"
	"os"
	"strings"
)

// ValidateRedirectURI applies the operator's host allowlist to a redirect URI.
//
// OAUTH_REDIRECT_HOSTS is a comma-separated list of exact hostnames; subdomains
// do not inherit trust. Loopback callbacks stay available for installed
// development clients whatever the list says.
//
// Upstream defaults this list to its own origin, because there the provider
// only ever redirects back to the platform itself. This fork is an SSO provider
// third parties register their own callbacks with, so an unset list means "no
// host allowlist" rather than "only our host": what constrains a client here is
// the exact match against its own registered URIs, enforced at registration by
// developer_portal.validateRedirectURI and again at /oauth2/auth. An operator
// running a closed deployment sets the variable to get the stricter rule back.
func ValidateRedirectURI(raw string) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" || u.User != nil || u.Fragment != "" {
		return errors.New("invalid redirect URI")
	}
	host := strings.ToLower(u.Hostname())
	loopback := host == "localhost" || host == "127.0.0.1" || host == "::1"
	if u.Scheme != "https" && (u.Scheme != "http" || !loopback) {
		return errors.New("redirect URI must use HTTPS")
	}
	if loopback {
		return nil
	}
	allowed := strings.TrimSpace(os.Getenv("OAUTH_REDIRECT_HOSTS"))
	if allowed == "" {
		return nil
	}
	for _, candidate := range strings.Split(allowed, ",") {
		if strings.EqualFold(strings.TrimSpace(candidate), host) {
			return nil
		}
	}
	return errors.New("redirect URI host is not allowed")
}
