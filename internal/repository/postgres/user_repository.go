package postgres

import (
	"context"

	"eventbooker/internal/domain/model"
	"eventbooker/internal/repository"
	pgxdriver "github.com/wb-go/wbf/dbpg/pgx-driver"
)

type UserRepository struct {
	db *pgxdriver.Postgres
}

func NewUserRepository(db *pgxdriver.Postgres) repository.UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(ctx context.Context, user *model.User) error {
	query := `
		INSERT INTO users (email, name, role)
		VALUES ($1, $2, $3)
		RETURNING id, created_at
	`

	err := getQueryExecuter(ctx, r.db).QueryRow(ctx, query, user.Email, user.Name, user.Role).Scan(&user.ID, &user.CreatedAt)
	if err != nil {
		return err
	}

	return nil
}

func (r *UserRepository) GetByID(ctx context.Context, id int64) (*model.User, error) {
	query := `
		SELECT id, email, name, role, created_at
		FROM users
		WHERE id = $1
	`

	return scanUser(getQueryExecuter(ctx, r.db).QueryRow(ctx, query, id))
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	query := `
		SELECT id, email, name, role, created_at
		FROM users
		WHERE email = $1
	`

	return scanUser(getQueryExecuter(ctx, r.db).QueryRow(ctx, query, email))
}
