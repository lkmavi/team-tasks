package auth_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"golang.org/x/crypto/bcrypt"

	"github.com/lkmavi/team-tasks/internal/domain"
	. "github.com/lkmavi/team-tasks/internal/service/auth"
)

type mockUserSaver struct{ mock.Mock }

func (m *mockUserSaver) SaveUser(ctx context.Context, u domain.User) error {
	return m.Called(ctx, u).Error(0)
}

type mockUserGetter struct{ mock.Mock }

func (m *mockUserGetter) GetByEmail(ctx context.Context, email string) (domain.User, error) {
	args := m.Called(ctx, email)
	return args.Get(0).(domain.User), args.Error(1)
}

var (
	ctx = context.Background()
)

func newSvc(saver *mockUserSaver, getter *mockUserGetter, secret string) *Service {
	return New(saver, getter, secret, time.Hour)
}

var errStorage = errors.New("storage error")

func TestRegister_DuplicateEmail(t *testing.T) {
	saver := &mockUserSaver{}
	getter := &mockUserGetter{}
	getter.On("GetByEmail", mock.Anything, "dupe@example.com").
		Return(domain.User{ID: uuid.New()}, nil)

	err := newSvc(saver, getter, "secret").Register(ctx, "dupe@example.com", "Alice", "password123")

	assert.ErrorIs(t, err, domain.ErrConflict)
	saver.AssertNotCalled(t, "SaveUser")
}

func TestRegister_OK(t *testing.T) {
	saver := &mockUserSaver{}
	getter := &mockUserGetter{}
	getter.On("GetByEmail", mock.Anything, "new@example.com").
		Return(domain.User{}, domain.ErrNotFound)
	saver.On("SaveUser", mock.Anything, mock.AnythingOfType("domain.User")).
		Return(nil)

	err := newSvc(saver, getter, "secret").Register(ctx, "new@example.com", "Bob", "password123")

	assert.NoError(t, err)
	saver.AssertExpectations(t)
}

func TestLogin_WrongPassword(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("correct"), bcrypt.MinCost)
	getter := &mockUserGetter{}
	getter.On("GetByEmail", mock.Anything, "user@example.com").
		Return(domain.User{ID: uuid.New(), Password: string(hash)}, nil)

	_, err := newSvc(&mockUserSaver{}, getter, "secret").Login(ctx, "user@example.com", "wrong")

	assert.ErrorIs(t, err, domain.ErrUnauthorized)
}

func TestLogin_UnknownEmail(t *testing.T) {
	getter := &mockUserGetter{}
	getter.On("GetByEmail", mock.Anything, "ghost@example.com").
		Return(domain.User{}, domain.ErrNotFound)

	_, err := newSvc(&mockUserSaver{}, getter, "secret").Login(ctx, "ghost@example.com", "pass")

	assert.ErrorIs(t, err, domain.ErrUnauthorized)
}

func TestLogin_OK_ReturnsValidJWT(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("pass"), bcrypt.MinCost)
	userID := uuid.New()
	getter := &mockUserGetter{}
	getter.On("GetByEmail", mock.Anything, "user@example.com").
		Return(domain.User{ID: userID, Password: string(hash)}, nil)

	token, err := newSvc(&mockUserSaver{}, getter, "supersecret").Login(ctx, "user@example.com", "pass")

	assert.NoError(t, err)
	assert.NotEmpty(t, token)

	parsed, parseErr := jwt.Parse(token, func(t *jwt.Token) (any, error) {
		return []byte("supersecret"), nil
	})
	assert.NoError(t, parseErr)
	assert.True(t, parsed.Valid)

	sub, _ := parsed.Claims.GetSubject()
	assert.Equal(t, userID.String(), sub)
}

func TestRegister_StorageLookupError(t *testing.T) {
	saver := &mockUserSaver{}
	getter := &mockUserGetter{}
	getter.On("GetByEmail", mock.Anything, "user@example.com").
		Return(domain.User{}, errStorage)

	err := newSvc(saver, getter, "secret").Register(ctx, "user@example.com", "Bob", "pass")

	assert.Error(t, err)
	assert.NotErrorIs(t, err, domain.ErrConflict)
	saver.AssertNotCalled(t, "SaveUser")
}

func TestRegister_SaveUserFails(t *testing.T) {
	saver := &mockUserSaver{}
	getter := &mockUserGetter{}
	getter.On("GetByEmail", mock.Anything, "new@example.com").
		Return(domain.User{}, domain.ErrNotFound)
	saver.On("SaveUser", mock.Anything, mock.AnythingOfType("domain.User")).
		Return(errStorage)

	err := newSvc(saver, getter, "secret").Register(ctx, "new@example.com", "Bob", "pass")

	assert.Error(t, err)
}

func TestLogin_StorageLookupError(t *testing.T) {
	getter := &mockUserGetter{}
	getter.On("GetByEmail", mock.Anything, "user@example.com").
		Return(domain.User{}, errStorage)

	_, err := newSvc(&mockUserSaver{}, getter, "secret").Login(ctx, "user@example.com", "pass")

	assert.Error(t, err)
	assert.NotErrorIs(t, err, domain.ErrUnauthorized)
}
