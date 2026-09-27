package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/thehien0811/chatter/internal/user/domain"
)

// UserRepository persists users in PostgreSQL according to the README schema.
type UserRepository struct {
	db *pgxpool.Pool
}

func NewUserRepository(db *pgxpool.Pool) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(ctx context.Context, user *domain.User) error {
	query := `
		INSERT INTO users (username, display_name, email, password_hash, status_message, presence, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
		RETURNING id, created_at, updated_at
	`

	return r.db.QueryRow(ctx, query,
		user.Username,
		user.DisplayName,
		user.Email,
		user.PasswordHash,
		user.StatusMessage,
		user.Presence,
	).Scan(&user.ID, &user.CreatedAt, &user.UpdatedAt)
}

func (r *UserRepository) GetByUsername(ctx context.Context, username string) (*domain.User, error) {
	query := `
		SELECT id, username, display_name, email, password_hash, avatar_media_id, status_message, presence, last_seen_at, created_at, updated_at, deleted_at
		FROM users
		WHERE username = $1 AND deleted_at IS NULL
	`

	user := &domain.User{}
	var email, avatarMediaID, statusMessage sql.NullString

	err := r.db.QueryRow(ctx, query, username).Scan(
		&user.ID,
		&user.Username,
		&user.DisplayName,
		&email,
		&user.PasswordHash,
		&avatarMediaID,
		&statusMessage,
		&user.Presence,
		&user.LastSeenAt,
		&user.CreatedAt,
		&user.UpdatedAt,
		&user.DeletedAt,
	)
	if err != nil {
		fmt.Println("errors", err)
		return nil, err
	}

	if email.Valid {
		user.Email = email.String
	}
	if avatarMediaID.Valid {
		user.AvatarMediaID = avatarMediaID.String
	}
	if statusMessage.Valid {
		user.StatusMessage = statusMessage.String
	}

	fmt.Println("UserRepository.GetByUsername: user found:", user.Username)
	return user, nil
}

func (r *UserRepository) GetByID(ctx context.Context, id string) (*domain.User, error) {
	query := `
		SELECT id, username, display_name, email, password_hash, avatar_media_id, status_message, presence, last_seen_at, created_at, updated_at, deleted_at
		FROM users
		WHERE id = $1 AND deleted_at IS NULL
	`

	user := &domain.User{}
	var email, avatarMediaID, statusMessage sql.NullString

	err := r.db.QueryRow(ctx, query, id).Scan(
		&user.ID,
		&user.Username,
		&user.DisplayName,
		&email,
		&user.PasswordHash,
		&avatarMediaID,
		&statusMessage,
		&user.Presence,
		&user.LastSeenAt,
		&user.CreatedAt,
		&user.UpdatedAt,
		&user.DeletedAt,
	)
	if err != nil {
		return nil, err
	}

	if email.Valid {
		user.Email = email.String
	}
	if avatarMediaID.Valid {
		user.AvatarMediaID = avatarMediaID.String
	}
	if statusMessage.Valid {
		user.StatusMessage = statusMessage.String
	}

	return user, nil
}

func (r *UserRepository) UpdateProfile(ctx context.Context, user *domain.User) error {
	query := `
		UPDATE users
		SET display_name = $1,
		    status_message = $2,
		    email = $3,
		    updated_at = NOW()
		WHERE id = $4 AND deleted_at IS NULL
	`

	cmd, err := r.db.Exec(ctx, query, user.DisplayName, user.StatusMessage, user.Email, user.ID)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return errors.New("user not found")
	}
	return nil
}

func (r *UserRepository) ExistsByUsername(ctx context.Context, username string) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM users WHERE username = $1 AND deleted_at IS NULL)`
	err := r.db.QueryRow(ctx, query, username).Scan(&exists)
	return exists, err
}

func (r *UserRepository) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM users WHERE email = $1 AND deleted_at IS NULL)`
	err := r.db.QueryRow(ctx, query, email).Scan(&exists)
	return exists, err
}

func (r *UserRepository) Ping(ctx context.Context) error {
	return r.db.Ping(ctx)
}

func NewPostgresPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("create pgx pool: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err = pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	return pool, nil
}
