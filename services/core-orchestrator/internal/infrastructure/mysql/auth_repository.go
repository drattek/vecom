package mysql

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"core-orchestrator/internal/domain"
)

var ErrAuthUserNotFound = errors.New("auth user not found")

type AuthRepository struct {
	db *sql.DB
}

type AuthUserCredentials struct {
	ID           int64
	Username     string
	Role         string
	IsActive     bool
	PasswordHash string
}

func NewAuthRepository(db *sql.DB) *AuthRepository {
	return &AuthRepository{db: db}
}

func (r *AuthRepository) FindUserCredentialsByUsername(username string) (*AuthUserCredentials, error) {
	query := `
		SELECT id, username, role, is_active, password_hash
		FROM ecom_api_user
		WHERE username = ?
		LIMIT 1
	`

	var credentials AuthUserCredentials
	err := r.db.QueryRow(query, username).Scan(
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

func (r *AuthRepository) StoreToken(jti string, userID int64, token string, issuedAt, expiresAt time.Time, userAgent, clientIP string) error {
	query := `
		INSERT INTO ecom_api_token (jti, user_id, token, issued_at, expires_at, user_agent, client_ip)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`

	_, err := r.db.Exec(query, jti, userID, token, issuedAt.UTC(), expiresAt.UTC(), userAgent, clientIP)
	if err != nil {
		return fmt.Errorf("error storing access token: %w", err)
	}

	return nil
}

func (r *AuthRepository) FindActiveUserByToken(jti, token string) (*domain.AuthUser, error) {
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
	err := r.db.QueryRow(query, jti, token).Scan(&user.ID, &user.Username, &user.Role, &user.IsActive)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrAuthUserNotFound
		}
		return nil, fmt.Errorf("error validating access token in database: %w", err)
	}

	return &user, nil
}
