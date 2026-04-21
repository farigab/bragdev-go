// Package repository contains persistence implementations backed by storage.
package repository

import (
	"github.com/jmoiron/sqlx"

	"github.com/farigab/bragdoc/internal/domain"
)

// RefreshTokenRepository defines operations for refresh tokens.
type RefreshTokenRepository interface {
	Save(t *domain.RefreshToken) (*domain.RefreshToken, error)
	FindByToken(token string) (*domain.RefreshToken, error)
	FindByUserLogin(userLogin string) ([]*domain.RefreshToken, error)
	Delete(t *domain.RefreshToken) error
	DeleteAllByUserLogin(userLogin string) error
	DeleteExpiredTokens() error
}

// PostgresRefreshTokenRepo is a Postgres-backed implementation of RefreshTokenRepository.
type PostgresRefreshTokenRepo struct {
	db *sqlx.DB
}

// NewPostgresRefreshTokenRepo constructs a PostgresRefreshTokenRepo with the provided DB.
func NewPostgresRefreshTokenRepo(db *sqlx.DB) *PostgresRefreshTokenRepo {
	return &PostgresRefreshTokenRepo{db: db}
}

// Save inserts or updates a refresh token record and returns the persisted entity.
func (r *PostgresRefreshTokenRepo) Save(t *domain.RefreshToken) (*domain.RefreshToken, error) {
	if t == nil {
		return nil, nil
	}
	var out domain.RefreshToken
	query := `INSERT INTO refresh_tokens (token, user_login, expires_at, created_at, revoked)
		VALUES ($1,$2,$3,$4,$5)
		ON CONFLICT (token) DO UPDATE SET
			user_login = EXCLUDED.user_login,
			expires_at = EXCLUDED.expires_at,
			created_at = EXCLUDED.created_at,
			revoked = EXCLUDED.revoked
		RETURNING token, user_login, expires_at, created_at, revoked`

	err := r.db.Get(&out, query, t.Token, t.UserLogin, t.ExpiresAt, t.CreatedAt, t.Revoked)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// FindByToken retrieves a refresh token by token string.
func (r *PostgresRefreshTokenRepo) FindByToken(token string) (*domain.RefreshToken, error) {
	var out domain.RefreshToken
	err := r.db.Get(&out, "SELECT token, user_login, expires_at, created_at, revoked FROM refresh_tokens WHERE token=$1", token)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// FindByUserLogin returns all refresh tokens for a given user login.
func (r *PostgresRefreshTokenRepo) FindByUserLogin(userLogin string) ([]*domain.RefreshToken, error) {
	var list []*domain.RefreshToken
	err := r.db.Select(&list, "SELECT token, user_login, expires_at, created_at, revoked FROM refresh_tokens WHERE user_login=$1", userLogin)
	if err != nil {
		return nil, err
	}
	return list, nil
}

// Delete removes the provided refresh token record.
func (r *PostgresRefreshTokenRepo) Delete(t *domain.RefreshToken) error {
	if t == nil {
		return nil
	}
	_, err := r.db.Exec("DELETE FROM refresh_tokens WHERE token=$1", t.Token)
	return err
}

// DeleteAllByUserLogin removes all refresh tokens for the given user.
func (r *PostgresRefreshTokenRepo) DeleteAllByUserLogin(userLogin string) error {
	_, err := r.db.Exec("DELETE FROM refresh_tokens WHERE user_login=$1", userLogin)
	return err
}

// DeleteExpiredTokens removes tokens past their expiration time.
func (r *PostgresRefreshTokenRepo) DeleteExpiredTokens() error {
	_, err := r.db.Exec("DELETE FROM refresh_tokens WHERE expires_at < NOW()")
	return err
}
