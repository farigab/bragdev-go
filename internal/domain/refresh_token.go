package domain

import "time"

type RefreshToken struct {
	Token     string    `db:"token" json:"token"`
	UserLogin string    `db:"user_login" json:"userLogin"`
	ExpiresAt time.Time `db:"expires_at" json:"expiresAt"`
	CreatedAt time.Time `db:"created_at" json:"createdAt"`
	Revoked   bool      `db:"revoked" json:"revoked"`
}

func NewRefreshToken(token, userLogin string, expiresAt time.Time) *RefreshToken {
	return &RefreshToken{Token: token, UserLogin: userLogin, ExpiresAt: expiresAt, CreatedAt: time.Now(), Revoked: false}
}
