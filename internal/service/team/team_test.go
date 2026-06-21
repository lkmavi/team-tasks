package team_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/lkmavi/team-tasks/internal/domain"
	. "github.com/lkmavi/team-tasks/internal/service/team"
)

type mockTeamStorage struct{ mock.Mock }

func (m *mockTeamStorage) CreateTeamWithOwner(ctx context.Context, team domain.Team, ownerID uuid.UUID) error {
	return m.Called(ctx, team, ownerID).Error(0)
}
func (m *mockTeamStorage) GetTeamsByUser(ctx context.Context, userID uuid.UUID) ([]domain.Team, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).([]domain.Team), args.Error(1)
}
func (m *mockTeamStorage) GetTeamByID(ctx context.Context, id uuid.UUID) (domain.Team, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(domain.Team), args.Error(1)
}
func (m *mockTeamStorage) SaveMember(ctx context.Context, teamID, userID uuid.UUID, role domain.Role) error {
	return m.Called(ctx, teamID, userID, role).Error(0)
}
func (m *mockTeamStorage) GetMemberRole(ctx context.Context, teamID, userID uuid.UUID) (domain.Role, error) {
	args := m.Called(ctx, teamID, userID)
	return args.Get(0).(domain.Role), args.Error(1)
}

type mockUserGetter struct{ mock.Mock }

func (m *mockUserGetter) GetByID(ctx context.Context, id uuid.UUID) (domain.User, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(domain.User), args.Error(1)
}

type mockNotifier struct{ mock.Mock }

func (m *mockNotifier) SendInvite(ctx context.Context, to, teamName string) error {
	return m.Called(ctx, to, teamName).Error(0)
}

var ctx = context.Background()

func newSvc(store *mockTeamStorage, users *mockUserGetter, notifier *mockNotifier) *Service {
	return New(store, store, users, notifier)
}

func TestInvite_CallerIsMember_Forbidden(t *testing.T) {
	store := &mockTeamStorage{}
	users := &mockUserGetter{}
	notifier := &mockNotifier{}

	teamID, callerID, targetID := uuid.New(), uuid.New(), uuid.New()

	store.On("GetTeamByID", mock.Anything, teamID).
		Return(domain.Team{ID: teamID, Name: "Acme"}, nil)
	store.On("GetMemberRole", mock.Anything, teamID, callerID).
		Return(domain.RoleMember, nil)

	err := newSvc(store, users, notifier).Invite(ctx, callerID, teamID, targetID)

	assert.ErrorIs(t, err, domain.ErrForbidden)
	notifier.AssertNotCalled(t, "SendInvite")
}

func TestInvite_CallerNotInTeam_Forbidden(t *testing.T) {
	store := &mockTeamStorage{}
	users := &mockUserGetter{}
	notifier := &mockNotifier{}

	teamID, callerID, targetID := uuid.New(), uuid.New(), uuid.New()

	store.On("GetTeamByID", mock.Anything, teamID).
		Return(domain.Team{ID: teamID, Name: "Acme"}, nil)
	store.On("GetMemberRole", mock.Anything, teamID, callerID).
		Return(domain.Role(""), domain.ErrNotFound)

	err := newSvc(store, users, notifier).Invite(ctx, callerID, teamID, targetID)

	assert.ErrorIs(t, err, domain.ErrForbidden)
}

func TestInvite_OK_SendInviteCalled(t *testing.T) {
	store := &mockTeamStorage{}
	users := &mockUserGetter{}
	notifier := &mockNotifier{}

	teamID, callerID, targetID := uuid.New(), uuid.New(), uuid.New()

	store.On("GetTeamByID", mock.Anything, teamID).
		Return(domain.Team{ID: teamID, Name: "Acme"}, nil)
	store.On("GetMemberRole", mock.Anything, teamID, callerID).
		Return(domain.RoleOwner, nil)
	users.On("GetByID", mock.Anything, targetID).
		Return(domain.User{ID: targetID, Email: "target@example.com"}, nil)
	store.On("SaveMember", mock.Anything, teamID, targetID, domain.RoleMember).
		Return(nil)
	notifier.On("SendInvite", mock.Anything, "target@example.com", "Acme").
		Return(nil)

	err := newSvc(store, users, notifier).Invite(ctx, callerID, teamID, targetID)

	assert.NoError(t, err)
	notifier.AssertCalled(t, "SendInvite", mock.Anything, "target@example.com", "Acme")
}

func TestInvite_NotifierError_DoesNotBlockInvitation(t *testing.T) {
	store := &mockTeamStorage{}
	users := &mockUserGetter{}
	notifier := &mockNotifier{}

	teamID, callerID, targetID := uuid.New(), uuid.New(), uuid.New()

	store.On("GetTeamByID", mock.Anything, teamID).
		Return(domain.Team{ID: teamID, Name: "Acme"}, nil)
	store.On("GetMemberRole", mock.Anything, teamID, callerID).
		Return(domain.RoleAdmin, nil)
	users.On("GetByID", mock.Anything, targetID).
		Return(domain.User{ID: targetID, Email: "target@example.com"}, nil)
	store.On("SaveMember", mock.Anything, teamID, targetID, domain.RoleMember).
		Return(nil)
	notifier.On("SendInvite", mock.Anything, "target@example.com", "Acme").
		Return(errors.New("circuit open"))

	err := newSvc(store, users, notifier).Invite(ctx, callerID, teamID, targetID)

	assert.NoError(t, err, "notifier failure must not fail the invitation")
}

func TestCreate_OK(t *testing.T) {
	store := &mockTeamStorage{}
	users := &mockUserGetter{}
	notifier := &mockNotifier{}

	ownerID := uuid.New()
	store.On("CreateTeamWithOwner", mock.Anything, mock.AnythingOfType("domain.Team"), ownerID).Return(nil)

	team, err := newSvc(store, users, notifier).Create(ctx, ownerID, "Dream Team")

	assert.NoError(t, err)
	assert.Equal(t, "Dream Team", team.Name)
	assert.Equal(t, ownerID, team.CreatedBy)
	store.AssertExpectations(t)
}

func TestCreate_StorageFails(t *testing.T) {
	store := &mockTeamStorage{}
	users := &mockUserGetter{}
	notifier := &mockNotifier{}

	ownerID := uuid.New()
	store.On("CreateTeamWithOwner", mock.Anything, mock.AnythingOfType("domain.Team"), ownerID).
		Return(errors.New("db error"))

	_, err := newSvc(store, users, notifier).Create(ctx, ownerID, "Dream Team")

	assert.Error(t, err)
}

func TestListForUser_OK(t *testing.T) {
	store := &mockTeamStorage{}
	users := &mockUserGetter{}
	notifier := &mockNotifier{}

	userID := uuid.New()
	expected := []domain.Team{{ID: uuid.New(), Name: "Alpha"}, {ID: uuid.New(), Name: "Beta"}}
	store.On("GetTeamsByUser", mock.Anything, userID).Return(expected, nil)

	teams, err := newSvc(store, users, notifier).ListForUser(ctx, userID)

	assert.NoError(t, err)
	assert.Len(t, teams, 2)
}

func TestListForUser_StorageFails(t *testing.T) {
	store := &mockTeamStorage{}
	users := &mockUserGetter{}
	notifier := &mockNotifier{}

	userID := uuid.New()
	store.On("GetTeamsByUser", mock.Anything, userID).
		Return([]domain.Team(nil), errors.New("db error"))

	_, err := newSvc(store, users, notifier).ListForUser(ctx, userID)

	assert.Error(t, err)
}

func TestInvite_GetTeamFails(t *testing.T) {
	store := &mockTeamStorage{}
	users := &mockUserGetter{}
	notifier := &mockNotifier{}

	teamID, callerID, targetID := uuid.New(), uuid.New(), uuid.New()
	store.On("GetTeamByID", mock.Anything, teamID).
		Return(domain.Team{}, errors.New("db error"))

	err := newSvc(store, users, notifier).Invite(ctx, callerID, teamID, targetID)

	assert.Error(t, err)
}

func TestInvite_GetMemberRoleFails(t *testing.T) {
	store := &mockTeamStorage{}
	users := &mockUserGetter{}
	notifier := &mockNotifier{}

	teamID, callerID, targetID := uuid.New(), uuid.New(), uuid.New()
	store.On("GetTeamByID", mock.Anything, teamID).
		Return(domain.Team{ID: teamID, Name: "X"}, nil)
	store.On("GetMemberRole", mock.Anything, teamID, callerID).
		Return(domain.Role(""), errors.New("db error"))

	err := newSvc(store, users, notifier).Invite(ctx, callerID, teamID, targetID)

	assert.Error(t, err)
	assert.NotErrorIs(t, err, domain.ErrForbidden)
}

func TestInvite_GetTargetUserFails(t *testing.T) {
	store := &mockTeamStorage{}
	users := &mockUserGetter{}
	notifier := &mockNotifier{}

	teamID, callerID, targetID := uuid.New(), uuid.New(), uuid.New()
	store.On("GetTeamByID", mock.Anything, teamID).
		Return(domain.Team{ID: teamID, Name: "X"}, nil)
	store.On("GetMemberRole", mock.Anything, teamID, callerID).
		Return(domain.RoleOwner, nil)
	users.On("GetByID", mock.Anything, targetID).
		Return(domain.User{}, errors.New("db error"))

	err := newSvc(store, users, notifier).Invite(ctx, callerID, teamID, targetID)

	assert.Error(t, err)
	notifier.AssertNotCalled(t, "SendInvite")
}

func TestInvite_SaveMemberFails(t *testing.T) {
	store := &mockTeamStorage{}
	users := &mockUserGetter{}
	notifier := &mockNotifier{}

	teamID, callerID, targetID := uuid.New(), uuid.New(), uuid.New()
	store.On("GetTeamByID", mock.Anything, teamID).
		Return(domain.Team{ID: teamID, Name: "X"}, nil)
	store.On("GetMemberRole", mock.Anything, teamID, callerID).
		Return(domain.RoleOwner, nil)
	users.On("GetByID", mock.Anything, targetID).
		Return(domain.User{ID: targetID, Email: "t@x.com"}, nil)
	store.On("SaveMember", mock.Anything, teamID, targetID, domain.RoleMember).
		Return(errors.New("db error"))

	err := newSvc(store, users, notifier).Invite(ctx, callerID, teamID, targetID)

	assert.Error(t, err)
	notifier.AssertNotCalled(t, "SendInvite")
}

func TestInvite_NotifierFails_NonFatal(t *testing.T) {
	store := &mockTeamStorage{}
	users := &mockUserGetter{}
	notifier := &mockNotifier{}

	teamID, callerID, targetID := uuid.New(), uuid.New(), uuid.New()
	store.On("GetTeamByID", mock.Anything, teamID).
		Return(domain.Team{ID: teamID, Name: "X"}, nil)
	store.On("GetMemberRole", mock.Anything, teamID, callerID).
		Return(domain.RoleOwner, nil)
	users.On("GetByID", mock.Anything, targetID).
		Return(domain.User{ID: targetID, Email: "t@x.com"}, nil)
	store.On("SaveMember", mock.Anything, teamID, targetID, domain.RoleMember).
		Return(nil)
	notifier.On("SendInvite", mock.Anything, "t@x.com", "X").
		Return(errors.New("smtp down"))

	err := newSvc(store, users, notifier).Invite(ctx, callerID, teamID, targetID)

	assert.NoError(t, err, "notifier failure must be non-fatal")
}
