package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/lkmavi/team-tasks/internal/domain"
	"github.com/lkmavi/team-tasks/pkg/slogx"
)

// UserSaver persists a new user record.
type UserSaver interface {
	SaveUser(ctx context.Context, u domain.User) error
}

// UserGetter retrieves a user by email.
type UserGetter interface {
	GetByEmail(ctx context.Context, email string) (domain.User, error)
}

// Service handles registration and authentication.
type Service struct {
	userSaver  UserSaver
	userGetter UserGetter
	secret     string
	expiry     time.Duration
}

// New creates an auth Service with the given storage adapters and JWT config.
func New(
	userSaver UserSaver,
	userGetter UserGetter,
	secret string,
	expiry time.Duration,
) *Service {
	return &Service{
		userSaver:  userSaver,
		userGetter: userGetter,
		secret:     secret,
		expiry:     expiry,
	}
}

func (s *Service) Register(ctx context.Context, email, name, password string) error {
	const op = "auth.Service.Register"

	log := slogx.FromContext(ctx).With(
		slog.String("op", op),
		slog.String("email", email),
	)

	log.Info("registering user")

	_, err := s.userGetter.GetByEmail(ctx, email)
	if err == nil {
		log.Warn("email already registered")
		return fmt.Errorf("%w: email already registered", domain.ErrConflict)
	}
	if !errors.Is(err, domain.ErrNotFound) {
		log.Error("failed to look up user by email", slogx.Err(err))
		return fmt.Errorf("%s: get by email: %w", op, err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Error("failed to hash password", slogx.Err(err))
		return fmt.Errorf("%s: hash password: %w", op, err)
	}

	id, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("%s: generate user id: %w", op, err)
	}

	if err = s.userSaver.SaveUser(ctx, domain.User{
		ID:        id,
		Email:     email,
		Name:      name,
		Password:  string(hash),
		CreatedAt: time.Now().UTC(),
	}); err != nil {
		log.Error("failed to save user", slogx.Err(err))
		return fmt.Errorf("%s: save user: %w", op, err)
	}

	log.Info("user registered successfully")
	return nil
}

func (s *Service) Login(ctx context.Context, email, password string) (string, error) {
	const op = "auth.Service.Login"

	log := slogx.FromContext(ctx).With(
		slog.String("op", op),
		slog.String("email", email),
	)

	log.Info("authenticating user")

	user, err := s.userGetter.GetByEmail(ctx, email)
	if errors.Is(err, domain.ErrNotFound) {
		log.Warn("user not found")
		return "", domain.ErrUnauthorized
	}
	if err != nil {
		log.Error("failed to get user by email", slogx.Err(err))
		return "", fmt.Errorf("%s: get by email: %w", op, err)
	}

	if err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		log.Warn("invalid password")
		return "", domain.ErrUnauthorized
	}

	claims := jwt.MapClaims{
		"sub": user.ID.String(),
		"exp": time.Now().Add(s.expiry).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(s.secret))
	if err != nil {
		log.Error("failed to sign token", slogx.Err(err))
		return "", fmt.Errorf("%s: sign token: %w", op, err)
	}

	log.Info("user authenticated successfully", slog.String("user_id", user.ID.String()))
	return signed, nil
}
