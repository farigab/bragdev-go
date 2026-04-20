package repository

import (
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/farigab/bragdoc/internal/domain"
)

// AchievementRepository defines persistence operations for achievements.
type AchievementRepository interface {
	Save(a *domain.Achievement) (*domain.Achievement, error)
	FindByID(id int64, userLogin string) (*domain.Achievement, error)
	FindByUser(userLogin string) ([]*domain.Achievement, error)
	Delete(id int64, userLogin string) error
	ExistsByUserAndDateAndTitle(userLogin string, date time.Time, title string) (bool, error)
}

// PostgresAchievementRepo is a sqlx-based implementation (skeleton).
type PostgresAchievementRepo struct {
	db *sqlx.DB
}

func NewPostgresAchievementRepo(db *sqlx.DB) *PostgresAchievementRepo {
	return &PostgresAchievementRepo{db: db}
}

func (r *PostgresAchievementRepo) Save(a *domain.Achievement) (*domain.Achievement, error) {
	if a == nil {
		return nil, nil
	}

	if a.ID == 0 {
		// insert
		var id int64
		err := r.db.QueryRow("INSERT INTO achievements (title, description, category, date, user_login) VALUES ($1,$2,$3,$4,$5) RETURNING id",
			a.Title, a.Description, a.Category, a.Date, a.UserLogin).Scan(&id)
		if err != nil {
			return nil, err
		}
		a.ID = id
		return a, nil
	}

	// update (ensure user owns the achievement)
	_, err := r.db.Exec("UPDATE achievements SET title=$1, description=$2, category=$3, date=$4 WHERE id=$5 AND user_login=$6",
		a.Title, a.Description, a.Category, a.Date, a.ID, a.UserLogin)
	if err != nil {
		return nil, err
	}
	return a, nil
}

func (r *PostgresAchievementRepo) FindByID(id int64, userLogin string) (*domain.Achievement, error) {
	var a domain.Achievement
	err := r.db.Get(&a, "SELECT id, title, description, category, date, user_login FROM achievements WHERE id=$1 AND user_login=$2", id, userLogin)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *PostgresAchievementRepo) FindByUser(userLogin string) ([]*domain.Achievement, error) {
	var list []*domain.Achievement
	err := r.db.Select(&list, "SELECT id, title, description, category, date, user_login FROM achievements WHERE user_login=$1 ORDER BY date DESC", userLogin)
	if err != nil {
		return nil, err
	}
	return list, nil
}

func (r *PostgresAchievementRepo) Delete(id int64, userLogin string) error {
	_, err := r.db.Exec("DELETE FROM achievements WHERE id=$1 AND user_login=$2", id, userLogin)
	return err
}

func (r *PostgresAchievementRepo) ExistsByUserAndDateAndTitle(userLogin string, date time.Time, title string) (bool, error) {
	var exists bool
	err := r.db.Get(&exists, "SELECT EXISTS(SELECT 1 FROM achievements WHERE user_login=$1 AND date=$2 AND title=$3)", userLogin, date, title)
	if err != nil {
		return false, err
	}
	return exists, nil
}
