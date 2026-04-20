package domain

import "time"

type Achievement struct {
	ID          int64     `db:"id" json:"id"`
	Title       string    `db:"title" json:"title"`
	Description string    `db:"description" json:"description"`
	Category    string    `db:"category" json:"category"`
	Date        time.Time `db:"date" json:"date"`
	UserLogin   string    `db:"user_login" json:"userLogin"`
}

func NewAchievement(title, description, category string, date time.Time, userLogin string) *Achievement {
	return &Achievement{
		Title:       title,
		Description: description,
		Category:    category,
		Date:        date,
		UserLogin:   userLogin,
	}
}
