package postgres

import (
	"context"

	"eventbooker/internal/domain/model"
	"eventbooker/internal/logging"
	"eventbooker/internal/repository"
	pgxdriver "github.com/wb-go/wbf/dbpg/pgx-driver"
)

type UserRepository struct {
	logger *logging.EventBookerLogger
	db     *pgxdriver.Postgres
}

func NewUserRepository(logger *logging.EventBookerLogger, db *pgxdriver.Postgres) repository.UserRepository {
	return &UserRepository{logger: logger, db: db}
}

func (r *UserRepository) Create(ctx context.Context, user *model.User) error {
	query := `
		INSERT INTO users (email, name, password_hash, role)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at
	`

	err := getQueryExecuter(ctx, r.db).QueryRow(ctx, query, user.Email, user.Name, user.PasswordHash, user.Role).Scan(&user.ID, &user.CreatedAt)
	if err != nil {
		r.logger.Ctx(ctx).Errorw("failed to create user in postgres", "email", user.Email, "error", err)
		return err
	}

	return nil
}

func (r *UserRepository) GetByID(ctx context.Context, id int64) (*model.User, error) {
	query := `
		SELECT id, email, name, password_hash, role, created_at
		FROM users
		WHERE id = $1
	`

	user, err := scanUser(getQueryExecuter(ctx, r.db).QueryRow(ctx, query, id))
	if err != nil {
		r.logger.Ctx(ctx).Debugw("failed to get user by id from postgres", "user_id", id, "error", err)
		return nil, err
	}

	return user, nil
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	query := `
		SELECT id, email, name, password_hash, role, created_at
		FROM users
		WHERE email = $1
	`

	user, err := scanUser(getQueryExecuter(ctx, r.db).QueryRow(ctx, query, email))
	if err != nil {
		r.logger.Ctx(ctx).Debugw("failed to get user by email from postgres", "email", email, "error", err)
		return nil, err
	}

	return user, nil
}
