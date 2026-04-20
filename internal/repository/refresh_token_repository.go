package repository

import (
	"github.com/farigab/bragdoc/internal/domain"
	"github.com/jmoiron/sqlx"
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

type PostgresRefreshTokenRepo struct {
	db *sqlx.DB
}

func NewPostgresRefreshTokenRepo(db *sqlx.DB) *PostgresRefreshTokenRepo {
	return &PostgresRefreshTokenRepo{db: db}
}

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

func (r *PostgresRefreshTokenRepo) FindByToken(token string) (*domain.RefreshToken, error) {
	var out domain.RefreshToken
	err := r.db.Get(&out, "SELECT token, user_login, expires_at, created_at, revoked FROM refresh_tokens WHERE token=$1", token)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *PostgresRefreshTokenRepo) FindByUserLogin(userLogin string) ([]*domain.RefreshToken, error) {
	var list []*domain.RefreshToken
	err := r.db.Select(&list, "SELECT token, user_login, expires_at, created_at, revoked FROM refresh_tokens WHERE user_login=$1", userLogin)
	if err != nil {
		return nil, err
	}
	return list, nil
}

func (r *PostgresRefreshTokenRepo) Delete(t *domain.RefreshToken) error {
	if t == nil {
		return nil
	}
	_, err := r.db.Exec("DELETE FROM refresh_tokens WHERE token=$1", t.Token)
	return err
}

func (r *PostgresRefreshTokenRepo) DeleteAllByUserLogin(userLogin string) error {
	_, err := r.db.Exec("DELETE FROM refresh_tokens WHERE user_login=$1", userLogin)
	return err
}

func (r *PostgresRefreshTokenRepo) DeleteExpiredTokens() error {
	_, err := r.db.Exec("DELETE FROM refresh_tokens WHERE expires_at < NOW()")
	return err
}
