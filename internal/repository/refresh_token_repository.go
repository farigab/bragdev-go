package repository

import (
	"context"
	"fmt"
	"time"

	sqlitecloud "github.com/sqlitecloud/sqlitecloud-go"

	"github.com/farigab/bragdev-go/internal/domain"
)

// RefreshTokenRepository defines operations for refresh tokens.
type RefreshTokenRepository interface {
	Save(ctx context.Context, t *domain.RefreshToken) (*domain.RefreshToken, error)
	FindByToken(ctx context.Context, token string) (*domain.RefreshToken, error)
	FindByUserLogin(ctx context.Context, userLogin string) ([]*domain.RefreshToken, error)
	Delete(ctx context.Context, t *domain.RefreshToken) error
	DeleteAllByUserLogin(ctx context.Context, userLogin string) error
	DeleteExpiredTokens(ctx context.Context) error
}

// SQLiteCloudRefreshTokenRepo implements RefreshTokenRepository using SQLite Cloud.
type SQLiteCloudRefreshTokenRepo struct {
	db *sqlitecloud.SQCloud
}

// NewRefreshTokenRepo creates a new SQLiteCloudRefreshTokenRepo.
func NewRefreshTokenRepo(db *sqlitecloud.SQCloud) *SQLiteCloudRefreshTokenRepo {
	return &SQLiteCloudRefreshTokenRepo{db: db}
}

// NewPostgresRefreshTokenRepo is an alias kept for backward compatibility.
func NewPostgresRefreshTokenRepo(db *sqlitecloud.SQCloud) *SQLiteCloudRefreshTokenRepo {
	return NewRefreshTokenRepo(db)
}

// sqliteTimeLayout stores timestamps in the format SQLite's datetime() understands
// so that lexicographic and datetime() comparisons both work correctly.
// RFC3339 uses 'T' and 'Z' which are NOT recognized by SQLite's datetime() function,
// causing silent failures in date comparisons (e.g. DeleteExpiredTokens).
const sqliteTimeLayout = "2006-01-02 15:04:05"

// scanRefreshToken reads a RefreshToken from result at the given row index.
// Column order: token(0), user_login(1), expires_at(2), created_at(3), revoked(4).
func scanRefreshToken(result *sqlitecloud.Result, row uint64) (*domain.RefreshToken, error) {
	token, _ := result.GetStringValue(row, 0)
	userLogin, _ := result.GetStringValue(row, 1)
	expiresAtStr, _ := result.GetStringValue(row, 2)
	createdAtStr, _ := result.GetStringValue(row, 3)
	revokedInt, _ := result.GetInt64Value(row, 4)

	expiresAt, _ := time.ParseInLocation(sqliteTimeLayout, expiresAtStr, time.UTC)
	createdAt, _ := time.ParseInLocation(sqliteTimeLayout, createdAtStr, time.UTC)

	return &domain.RefreshToken{
		Token:     token,
		UserLogin: userLogin,
		ExpiresAt: expiresAt,
		CreatedAt: createdAt,
		Revoked:   revokedInt != 0,
	}, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// Save inserts or updates a refresh token and returns the persisted entity.
func (r *SQLiteCloudRefreshTokenRepo) Save(ctx context.Context, t *domain.RefreshToken) (*domain.RefreshToken, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if t == nil {
		return nil, nil
	}
	q := fmt.Sprintf(`INSERT INTO refresh_tokens (token, user_login, expires_at, created_at, revoked)
VALUES ('%s', '%s', '%s', '%s', %d)
ON CONFLICT(token) DO UPDATE SET
  user_login = excluded.user_login,
  expires_at = excluded.expires_at,
  created_at = excluded.created_at,
  revoked    = excluded.revoked
RETURNING token, user_login, expires_at, created_at, revoked;`,
		sqlEscape(t.Token),
		sqlEscape(t.UserLogin),
		t.ExpiresAt.UTC().Format(sqliteTimeLayout),
		t.CreatedAt.UTC().Format(sqliteTimeLayout),
		boolToInt(t.Revoked),
	)
	result, err := r.db.Select(q)
	if err != nil {
		return nil, err
	}
	if result == nil || result.GetNumberOfRows() == 0 {
		return r.FindByToken(ctx, t.Token)
	}
	return scanRefreshToken(result, 0)
}

// FindByToken retrieves a refresh token by its value.
func (r *SQLiteCloudRefreshTokenRepo) FindByToken(ctx context.Context, token string) (*domain.RefreshToken, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	q := fmt.Sprintf(
		"SELECT token, user_login, expires_at, created_at, revoked FROM refresh_tokens WHERE token='%s';",
		sqlEscape(token),
	)
	result, err := r.db.Select(q)
	if err != nil {
		return nil, err
	}
	if result == nil || result.GetNumberOfRows() == 0 {
		return nil, fmt.Errorf("refresh token not found")
	}
	return scanRefreshToken(result, 0)
}

// FindByUserLogin returns all refresh tokens for a given user login.
func (r *SQLiteCloudRefreshTokenRepo) FindByUserLogin(ctx context.Context, userLogin string) ([]*domain.RefreshToken, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	q := fmt.Sprintf(
		"SELECT token, user_login, expires_at, created_at, revoked FROM refresh_tokens WHERE user_login='%s';",
		sqlEscape(userLogin),
	)
	result, err := r.db.Select(q)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, nil
	}
	list := make([]*domain.RefreshToken, 0, result.GetNumberOfRows())
	for row := uint64(0); row < result.GetNumberOfRows(); row++ {
		rt, err := scanRefreshToken(result, row)
		if err != nil {
			return nil, err
		}
		list = append(list, rt)
	}
	return list, nil
}

// Delete removes the provided refresh token record.
func (r *SQLiteCloudRefreshTokenRepo) Delete(ctx context.Context, t *domain.RefreshToken) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if t == nil {
		return nil
	}
	q := fmt.Sprintf(
		"DELETE FROM refresh_tokens WHERE token='%s';",
		sqlEscape(t.Token),
	)
	return r.db.Execute(q)
}

// DeleteAllByUserLogin removes all refresh tokens for the given user.
func (r *SQLiteCloudRefreshTokenRepo) DeleteAllByUserLogin(ctx context.Context, userLogin string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	q := fmt.Sprintf(
		"DELETE FROM refresh_tokens WHERE user_login='%s';",
		sqlEscape(userLogin),
	)
	return r.db.Execute(q)
}

// DeleteExpiredTokens removes tokens past their expiration time.
// Timestamps are stored as "2006-01-02 15:04:05" (UTC) so SQLite's
// datetime() comparisons work correctly — RFC3339's 'T'/'Z' are not
// recognized by SQLite and would cause silent comparison failures.
func (r *SQLiteCloudRefreshTokenRepo) DeleteExpiredTokens(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return r.db.Execute("DELETE FROM refresh_tokens WHERE expires_at < datetime('now');")
}
