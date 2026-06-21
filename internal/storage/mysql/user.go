package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/lkmavi/team-tasks/internal/domain"
	"github.com/lkmavi/team-tasks/pkg/slogx"
)

// UserStorage implements user persistence backed by MySQL.
type UserStorage struct {
	db *sqlx.DB
}

// NewUserStorage creates a UserStorage using the provided database connection.
func NewUserStorage(db *sqlx.DB) *UserStorage {
	return &UserStorage{db: db}
}

type userRow struct {
	ID        []byte    `db:"id"`
	Email     string    `db:"email"`
	Name      string    `db:"name"`
	Password  string    `db:"password"`
	CreatedAt time.Time `db:"created_at"`
}

func (s *UserStorage) SaveUser(ctx context.Context, u domain.User) error {
	const op = "mysql.UserStorage.SaveUser"
	log := slogx.FromContext(ctx).With(slog.String("op", op), slog.String("email", u.Email))
	log.Info("saving user")

	_, err := s.db.ExecContext(ctx,
		"INSERT INTO users (id, email, name, password, created_at) VALUES (?, ?, ?, ?, ?)",
		uuidToBytes(u.ID), u.Email, u.Name, u.Password, u.CreatedAt,
	)
	if err != nil {
		if isDuplicateEntry(err) {
			log.Warn("user already exists")
			return domain.ErrConflict
		}
		log.Error("failed to save user", slogx.Err(err))
		return fmt.Errorf("mysql: save user: %w", err)
	}

	log.Info("user saved")
	return nil
}

func (s *UserStorage) GetByEmail(ctx context.Context, email string) (domain.User, error) {
	const op = "mysql.UserStorage.GetByEmail"
	log := slogx.FromContext(ctx).With(slog.String("op", op), slog.String("email", email))
	log.Info("fetching user by email")

	var row userRow
	err := s.db.GetContext(ctx, &row,
		"SELECT id, email, name, password, created_at FROM users WHERE email = ?",
		email,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			log.Warn("user not found")
			return domain.User{}, domain.ErrNotFound
		}
		log.Error("failed to fetch user by email", slogx.Err(err))
		return domain.User{}, fmt.Errorf("mysql: get user by email: %w", err)
	}

	u, err := toUser(row)
	if err != nil {
		log.Error("failed to parse user row", slogx.Err(err))
		return domain.User{}, err
	}

	log.Info("user fetched")
	return u, nil
}

func (s *UserStorage) GetByID(ctx context.Context, id uuid.UUID) (domain.User, error) {
	const op = "mysql.UserStorage.GetByID"
	log := slogx.FromContext(ctx).With(slog.String("op", op), slog.String("user_id", id.String()))
	log.Info("fetching user by id")

	var row userRow
	err := s.db.GetContext(ctx, &row,
		"SELECT id, email, name, password, created_at FROM users WHERE id = ?",
		uuidToBytes(id),
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			log.Warn("user not found")
			return domain.User{}, domain.ErrNotFound
		}
		log.Error("failed to fetch user by id", slogx.Err(err))
		return domain.User{}, fmt.Errorf("mysql: get user by id: %w", err)
	}

	u, err := toUser(row)
	if err != nil {
		log.Error("failed to parse user row", slogx.Err(err))
		return domain.User{}, err
	}

	log.Info("user fetched")
	return u, nil
}

func toUser(r userRow) (domain.User, error) {
	id, err := bytesToUUID(r.ID)
	if err != nil {
		return domain.User{}, fmt.Errorf("mysql: parse user id: %w", err)
	}
	return domain.User{ID: id, Email: r.Email, Name: r.Name, Password: r.Password, CreatedAt: r.CreatedAt}, nil
}
