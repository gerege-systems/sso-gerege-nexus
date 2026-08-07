package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SessionCookieName is the cookie carrying the opaque session token.
const SessionCookieName = "session_token"

// DefaultSessionTTL bounds how long a single login stays valid.
const DefaultSessionTTL = 12 * time.Hour

var ErrSessionInvalid = errors.New("session is invalid or expired")

// SessionStore issues and resolves opaque server-side session tokens.
//
// Tokens are 256 bits of crypto/rand output; only their SHA-256 digest is
// persisted, so a database leak cannot be replayed as a login.
type SessionStore struct {
	db  *pgxpool.Pool
	ttl time.Duration
}

func NewSessionStore(db *pgxpool.Pool, ttl time.Duration) *SessionStore {
	if ttl <= 0 {
		ttl = DefaultSessionTTL
	}
	return &SessionStore{db: db, ttl: ttl}
}

// TTL returns the configured session lifetime.
func (s *SessionStore) TTL() time.Duration { return s.ttl }

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func newToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate session token: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

// Create issues a new session and returns the plaintext token. The token is
// shown to the caller exactly once — only its digest is stored.
func (s *SessionStore) Create(ctx context.Context, userID, tenantID, authMethod, userAgent, ip string) (string, time.Time, error) {
	token, err := newToken()
	if err != nil {
		return "", time.Time{}, err
	}

	expiresAt := time.Now().Add(s.ttl)
	if authMethod == "" {
		authMethod = "password"
	}

	_, err = s.db.Exec(ctx,
		`INSERT INTO sessions (token_hash, user_id, tenant_id, auth_method, user_agent, ip_address, expires_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		hashToken(token), userID, tenantID, authMethod, userAgent, ip, expiresAt)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("persist session: %w", err)
	}

	return token, expiresAt, nil
}

// Resolve validates a token and returns the claims bound to it.
func (s *SessionStore) Resolve(ctx context.Context, token string) (UserClaims, error) {
	if token == "" {
		return UserClaims{}, ErrSessionInvalid
	}

	var claims UserClaims
	err := s.db.QueryRow(ctx,
		`SELECT s.user_id::text, s.tenant_id::text, u.email,
		        (u.is_admin OR EXISTS (
		            SELECT 1 FROM memberships m
		            JOIN membership_roles mr ON mr.membership_id=m.id
		            JOIN roles r ON r.id=mr.role_id
		            WHERE m.tenant_id=s.tenant_id AND m.user_id=s.user_id
		              AND r.tenant_id=s.tenant_id AND r.code='admin' AND r.active
		        )) AS is_admin
		   FROM sessions s
		   JOIN users u ON u.id = s.user_id
		  WHERE s.token_hash = $1
		    AND s.revoked_at IS NULL
		    AND s.expires_at > NOW()`,
		hashToken(token)).Scan(&claims.UserID, &claims.TenantID, &claims.Email, &claims.IsAdmin)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return UserClaims{}, ErrSessionInvalid
		}
		return UserClaims{}, fmt.Errorf("resolve session: %w", err)
	}

	return claims, nil
}

// Revoke invalidates a single session. Revoking an unknown token is a no-op so
// that logout never leaks whether a token existed.
func (s *SessionStore) Revoke(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	_, err := s.db.Exec(ctx,
		`UPDATE sessions SET revoked_at = NOW() WHERE token_hash = $1 AND revoked_at IS NULL`,
		hashToken(token))
	return err
}

// DeleteExpired purges sessions that can no longer be used.
func (s *SessionStore) DeleteExpired(ctx context.Context) (int64, error) {
	tag, err := s.db.Exec(ctx,
		`DELETE FROM sessions WHERE expires_at < NOW() - INTERVAL '7 days'`)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// TokenFromRequest extracts a session token from the cookie or the
// Authorization header, in that order.
func TokenFromRequest(r *http.Request) string {
	if cookie, err := r.Cookie(SessionCookieName); err == nil && cookie.Value != "" {
		return cookie.Value
	}
	const prefix = "Bearer "
	if header := r.Header.Get("Authorization"); len(header) > len(prefix) && header[:len(prefix)] == prefix {
		return header[len(prefix):]
	}
	return ""
}

// SetSessionCookie writes the session cookie with hardened attributes.
//
// SameSite is Lax rather than Strict because this deployment is an SSO
// provider: a relying party sends the browser to /oauth2/auth as a top-level
// cross-site navigation, and Strict withholds the cookie on exactly that
// request — every sign-in handoff would land on the login screen even for a
// user with a live session.
//
// Lax still withholds the cookie from cross-site POSTs and subresource loads,
// which is where CSRF lives. Nothing on this API changes state through a GET,
// so the difference costs no protection.
func SetSessionCookie(w http.ResponseWriter, token string, expiresAt time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    token,
		Path:     "/",
		Expires:  expiresAt,
		HttpOnly: true,
		Secure:   IsProduction(),
		SameSite: http.SameSiteLaxMode,
	})
}

// ClearSessionCookie expires the session cookie on the client.
func ClearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   IsProduction(),
		// Must match SetSessionCookie's attributes or the browser treats this
		// as a different cookie and leaves the original in place.
		SameSite: http.SameSiteLaxMode,
	})
}

// IsProduction reports whether the process runs with ENVIRONMENT=production.
func IsProduction() bool {
	return os.Getenv("ENVIRONMENT") == "production"
}
