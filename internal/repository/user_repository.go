// Package repository contains persistence implementations backed by storage.
package repository

import (
	"github.com/jmoiron/sqlx"

	"github.com/farigab/bragdev-go/internal/domain"
)

// UserRepository defines persistence for users.
type UserRepository interface {
	FindByLogin(login string) (*domain.User, error)
	Save(u *domain.User) (*domain.User, error)
	ExistsByLogin(login string) (bool, error)
	ClearGitHubToken(login string) error
}

// PostgresUserRepo implements UserRepository using Postgres via sqlx.
type PostgresUserRepo struct {
	db *sqlx.DB
}

// NewPostgresUserRepo creates a new PostgresUserRepo backed by the given DB.
func NewPostgresUserRepo(db *sqlx.DB) *PostgresUserRepo {
	return &PostgresUserRepo{db: db}
}

// FindByLogin looks up a user by login.
func (r *PostgresUserRepo) FindByLogin(login string) (*domain.User, error) {
	var u domain.User
	err := r.db.Get(&u, "SELECT login, name, avatar_url, github_access_token FROM users WHERE login=$1", login)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// Save inserts or updates a user record and returns the stored entity.
func (r *PostgresUserRepo) Save(u *domain.User) (*domain.User, error) {
	if u == nil {
		return nil, nil
	}
	var out domain.User
	query := `INSERT INTO users (login, name, avatar_url, github_access_token)
		VALUES ($1,$2,$3,$4)
		ON CONFLICT (login) DO UPDATE SET
			name = EXCLUDED.name,
			avatar_url = EXCLUDED.avatar_url,
			github_access_token = CASE WHEN EXCLUDED.github_access_token <> '' THEN EXCLUDED.github_access_token ELSE users.github_access_token END
		RETURNING login, name, avatar_url, github_access_token`

	err := r.db.Get(&out, query, u.Login, u.Name, u.AvatarURL, u.GitHubAccessToken)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// ExistsByLogin returns true when a user with the given login exists.
func (r *PostgresUserRepo) ExistsByLogin(login string) (bool, error) {
	var exists bool
	err := r.db.Get(&exists, "SELECT EXISTS(SELECT 1 FROM users WHERE login=$1)", login)
	if err != nil {
		return false, err
	}
	return exists, nil
}

// ClearGitHubToken clears the stored GitHub access token for the given user.
func (r *PostgresUserRepo) ClearGitHubToken(login string) error {
	_, err := r.db.Exec("UPDATE users SET github_access_token = '' WHERE login = $1", login)
	return err
}
