package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"core-orchestrator/internal/domain"
)

var ErrAuthUserNotFound = errors.New("auth user not found")

type AuthRepository struct {
	db Querier
}

type AuthUserCredentials struct {
	ID           int64
	Username     string
	Role         string
	IsActive     bool
	PasswordHash string
}

func NewAuthRepository(db Querier) *AuthRepository {
	return &AuthRepository{db: db}
}

func (r *AuthRepository) FindUserCredentialsByUsername(ctx context.Context, username string) (*AuthUserCredentials, error) {
	query := `
		SELECT id, username, role, is_active, password_hash
		FROM ecom_api_user
		WHERE username = ?
		LIMIT 1
	`

	var credentials AuthUserCredentials
	err := r.db.QueryRowContext(ctx, query, username).Scan(
		&credentials.ID,
		&credentials.Username,
		&credentials.Role,
		&credentials.IsActive,
		&credentials.PasswordHash,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrAuthUserNotFound
		}
		return nil, fmt.Errorf("error finding auth user credentials: %w", err)
	}

	return &credentials, nil
}

func (r *AuthRepository) StoreToken(ctx context.Context, jti string, userID int64, token string, issuedAt, expiresAt time.Time, userAgent, clientIP string) error {
	query := `
		INSERT INTO ecom_api_token (jti, user_id, token, issued_at, expires_at, user_agent, client_ip)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`

	_, err := r.db.ExecContext(ctx, query, jti, userID, token, issuedAt.UTC(), expiresAt.UTC(), userAgent, clientIP)
	if err != nil {
		return fmt.Errorf("error storing access token: %w", err)
	}

	return nil
}

// RevokeToken marks the access token identified by jti as revoked so it can no
// longer pass FindActiveUserByToken. Revoking an already-revoked, expired or
// unknown token is a no-op (no error) so logout stays idempotent.
func (r *AuthRepository) RevokeToken(ctx context.Context, jti, token string) error {
	query := `
		UPDATE ecom_api_token
		SET revoked_at = UTC_TIMESTAMP()
		WHERE jti = ?
		  AND token = ?
		  AND revoked_at IS NULL
	`

	_, err := r.db.ExecContext(ctx, query, jti, token)
	if err != nil {
		return fmt.Errorf("error revoking access token: %w", err)
	}

	return nil
}

func (r *AuthRepository) FindActiveUserByToken(ctx context.Context, jti, token string) (*domain.AuthUser, error) {
	query := `
		SELECT u.id, u.username, u.role, u.is_active
		FROM ecom_api_token t
		INNER JOIN ecom_api_user u ON u.id = t.user_id
		WHERE t.jti = ?
		  AND t.token = ?
		  AND t.revoked_at IS NULL
		  AND t.expires_at > UTC_TIMESTAMP()
		  AND u.is_active = 1
		LIMIT 1
	`

	var user domain.AuthUser
	err := r.db.QueryRowContext(ctx, query, jti, token).Scan(&user.ID, &user.Username, &user.Role, &user.IsActive)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrAuthUserNotFound
		}
		return nil, fmt.Errorf("error validating access token in database: %w", err)
	}

	return &user, nil
}
