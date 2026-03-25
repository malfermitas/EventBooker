package postgres

import (
	"context"
	"time"

	"eventbooker/internal/domain/model"
	"eventbooker/internal/repository"
	pqxdriver "github.com/wb-go/wbf/dbpg/pgx-driver"
)

type RefreshTokenRepository struct {
	db *pqxdriver.Postgres
}

func NewRefreshTokenRepository(db *pqxdriver.Postgres) repository.RefreshTokenRepository {
	return &RefreshTokenRepository{db: db}
}

func (r *RefreshTokenRepository) Create(ctx context.Context, token *model.RefreshToken) error {
	query := `
		INSERT INTO refresh_tokens (user_id, token_hash, expires_at, revoked_at, replaced_by_token_id, user_agent, ip_address)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at
	`

	err := getQueryExecuter(ctx, r.db).QueryRow(
		ctx,
		query,
		token.UserID,
		token.TokenHash,
		token.ExpiresAt,
		token.RevokedAt,
		token.ReplacedByTokenID,
		token.UserAgent,
		token.IPAddress,
	).Scan(&token.ID, &token.CreatedAt)
	if err != nil {
		return err
	}

	return nil
}

func (r *RefreshTokenRepository) GetByTokenHash(ctx context.Context, tokenHash string) (*model.RefreshToken, error) {
	query := `
		SELECT id, user_id, token_hash, expires_at, created_at, revoked_at, replaced_by_token_id, user_agent, ip_address
		FROM refresh_tokens
		WHERE token_hash = $1
	`

	return scanRefreshToken(getQueryExecuter(ctx, r.db).QueryRow(ctx, query, tokenHash))
}

func (r *RefreshTokenRepository) RevokeByID(ctx context.Context, id int64, revokedAt time.Time) error {
	query := `
		UPDATE refresh_tokens
		SET revoked_at = $2
		WHERE id = $1
	`

	_, err := getQueryExecuter(ctx, r.db).Exec(ctx, query, id, revokedAt)
	return err
}

func (r *RefreshTokenRepository) RevokeAndReplace(ctx context.Context, id int64, revokedAt time.Time, replacedByTokenID int64) error {
	query := `
		UPDATE refresh_tokens
		SET revoked_at = $2, replaced_by_token_id = $3
		WHERE id = $1
	`

	_, err := getQueryExecuter(ctx, r.db).Exec(ctx, query, id, revokedAt, replacedByTokenID)
	return err
}

func (r *RefreshTokenRepository) RevokeAllByUserID(ctx context.Context, userID int64, revokedAt time.Time) error {
	query := `
		UPDATE refresh_tokens
		SET revoked_at = $2
		WHERE user_id = $1 AND revoked_at IS NULL
	`

	_, err := getQueryExecuter(ctx, r.db).Exec(ctx, query, userID, revokedAt)
	return err
}
