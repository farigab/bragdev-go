package repository

import (
	"github.com/jmoiron/sqlx"
	"github.com/farigab/bragdoc/internal/domain"
)

// UserRepository defines persistence for users.
type UserRepository interface {
	FindByLogin(login string) (*domain.User, error)
	Save(u *domain.User) (*domain.User, error)
	ExistsByLogin(login string) (bool, error)
}

type PostgresUserRepo struct {
	db *sqlx.DB
}

func NewPostgresUserRepo(db *sqlx.DB) *PostgresUserRepo {
	return &PostgresUserRepo{db: db}
}

func (r *PostgresUserRepo) FindByLogin(login string) (*domain.User, error) {
	var u domain.User
	err := r.db.Get(&u, "SELECT login, name, avatar_url, github_access_token FROM users WHERE login=$1", login)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

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
			github_access_token = EXCLUDED.github_access_token
		RETURNING login, name, avatar_url, github_access_token`

	err := r.db.Get(&out, query, u.Login, u.Name, u.AvatarURL, u.GitHubAccessToken)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *PostgresUserRepo) ExistsByLogin(login string) (bool, error) {
	var exists bool
	err := r.db.Get(&exists, "SELECT EXISTS(SELECT 1 FROM users WHERE login=$1)", login)
	if err != nil {
		return false, err
	}
	return exists, nil
}
