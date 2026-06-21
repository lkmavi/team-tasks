package comment_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/lkmavi/team-tasks/internal/domain"
	. "github.com/lkmavi/team-tasks/internal/service/comment"
)

type mockSaver struct{ mock.Mock }

func (m *mockSaver) SaveComment(ctx context.Context, c domain.Comment) (domain.Comment, error) {
	args := m.Called(ctx, c)
	return args.Get(0).(domain.Comment), args.Error(1)
}

type mockLister struct{ mock.Mock }

func (m *mockLister) ListComments(ctx context.Context, taskID uuid.UUID) ([]domain.Comment, error) {
	args := m.Called(ctx, taskID)
	return args.Get(0).([]domain.Comment), args.Error(1)
}

type mockTaskGetter struct{ mock.Mock }

func (m *mockTaskGetter) GetTaskByID(ctx context.Context, id uuid.UUID) (domain.Task, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(domain.Task), args.Error(1)
}

type mockMemberChecker struct{ mock.Mock }

func (m *mockMemberChecker) GetMemberRole(ctx context.Context, teamID, userID uuid.UUID) (domain.Role, error) {
	args := m.Called(ctx, teamID, userID)
	return args.Get(0).(domain.Role), args.Error(1)
}

var ctx = context.Background()

func newSvc(saver *mockSaver, lister *mockLister, tasks *mockTaskGetter, members *mockMemberChecker) *Service {
	return New(saver, lister, tasks, members)
}

func TestAdd_OK(t *testing.T) {
	saver := &mockSaver{}
	lister := &mockLister{}
	tasks := &mockTaskGetter{}
	members := &mockMemberChecker{}

	taskID, teamID, callerID := uuid.New(), uuid.New(), uuid.New()
	tasks.On("GetTaskByID", mock.Anything, taskID).Return(domain.Task{ID: taskID, TeamID: teamID}, nil)
	members.On("GetMemberRole", mock.Anything, teamID, callerID).Return(domain.RoleMember, nil)
	saver.On("SaveComment", mock.Anything, mock.AnythingOfType("domain.Comment")).
		Return(domain.Comment{ID: 1, TaskID: taskID, UserID: callerID, Body: "hello"}, nil)

	c, err := newSvc(saver, lister, tasks, members).Add(ctx, callerID, taskID, "hello")
	assert.NoError(t, err)
	assert.Equal(t, "hello", c.Body)
	assert.EqualValues(t, 1, c.ID)
}

func TestAdd_TaskNotFound(t *testing.T) {
	saver := &mockSaver{}
	lister := &mockLister{}
	tasks := &mockTaskGetter{}
	members := &mockMemberChecker{}

	taskID, callerID := uuid.New(), uuid.New()
	tasks.On("GetTaskByID", mock.Anything, taskID).Return(domain.Task{}, domain.ErrNotFound)

	_, err := newSvc(saver, lister, tasks, members).Add(ctx, callerID, taskID, "x")
	assert.Error(t, err)
	saver.AssertNotCalled(t, "SaveComment")
}

func TestAdd_NotMember_Forbidden(t *testing.T) {
	saver := &mockSaver{}
	lister := &mockLister{}
	tasks := &mockTaskGetter{}
	members := &mockMemberChecker{}

	taskID, teamID, callerID := uuid.New(), uuid.New(), uuid.New()
	tasks.On("GetTaskByID", mock.Anything, taskID).Return(domain.Task{ID: taskID, TeamID: teamID}, nil)
	members.On("GetMemberRole", mock.Anything, teamID, callerID).Return(domain.Role(""), domain.ErrNotFound)

	_, err := newSvc(saver, lister, tasks, members).Add(ctx, callerID, taskID, "x")
	assert.ErrorIs(t, err, domain.ErrForbidden)
	saver.AssertNotCalled(t, "SaveComment")
}

func TestAdd_SaveFails(t *testing.T) {
	saver := &mockSaver{}
	lister := &mockLister{}
	tasks := &mockTaskGetter{}
	members := &mockMemberChecker{}

	taskID, teamID, callerID := uuid.New(), uuid.New(), uuid.New()
	tasks.On("GetTaskByID", mock.Anything, taskID).Return(domain.Task{ID: taskID, TeamID: teamID}, nil)
	members.On("GetMemberRole", mock.Anything, teamID, callerID).Return(domain.RoleMember, nil)
	saver.On("SaveComment", mock.Anything, mock.AnythingOfType("domain.Comment")).
		Return(domain.Comment{}, errors.New("db"))

	_, err := newSvc(saver, lister, tasks, members).Add(ctx, callerID, taskID, "x")
	assert.Error(t, err)
}

func TestList_OK(t *testing.T) {
	saver := &mockSaver{}
	lister := &mockLister{}
	tasks := &mockTaskGetter{}
	members := &mockMemberChecker{}

	taskID, teamID, callerID := uuid.New(), uuid.New(), uuid.New()
	tasks.On("GetTaskByID", mock.Anything, taskID).Return(domain.Task{ID: taskID, TeamID: teamID}, nil)
	members.On("GetMemberRole", mock.Anything, teamID, callerID).Return(domain.RoleMember, nil)
	lister.On("ListComments", mock.Anything, taskID).
		Return([]domain.Comment{{ID: 1, Body: "first"}}, nil)

	comments, err := newSvc(saver, lister, tasks, members).List(ctx, callerID, taskID)
	assert.NoError(t, err)
	assert.Len(t, comments, 1)
}

func TestList_TaskNotFound(t *testing.T) {
	saver := &mockSaver{}
	lister := &mockLister{}
	tasks := &mockTaskGetter{}
	members := &mockMemberChecker{}

	taskID, callerID := uuid.New(), uuid.New()
	tasks.On("GetTaskByID", mock.Anything, taskID).Return(domain.Task{}, domain.ErrNotFound)

	_, err := newSvc(saver, lister, tasks, members).List(ctx, callerID, taskID)
	assert.Error(t, err)
}

func TestList_NotMember_Forbidden(t *testing.T) {
	saver := &mockSaver{}
	lister := &mockLister{}
	tasks := &mockTaskGetter{}
	members := &mockMemberChecker{}

	taskID, teamID, callerID := uuid.New(), uuid.New(), uuid.New()
	tasks.On("GetTaskByID", mock.Anything, taskID).Return(domain.Task{ID: taskID, TeamID: teamID}, nil)
	members.On("GetMemberRole", mock.Anything, teamID, callerID).Return(domain.Role(""), domain.ErrNotFound)

	_, err := newSvc(saver, lister, tasks, members).List(ctx, callerID, taskID)
	assert.ErrorIs(t, err, domain.ErrForbidden)
}

func TestList_StorageFails(t *testing.T) {
	saver := &mockSaver{}
	lister := &mockLister{}
	tasks := &mockTaskGetter{}
	members := &mockMemberChecker{}

	taskID, teamID, callerID := uuid.New(), uuid.New(), uuid.New()
	tasks.On("GetTaskByID", mock.Anything, taskID).Return(domain.Task{ID: taskID, TeamID: teamID}, nil)
	members.On("GetMemberRole", mock.Anything, teamID, callerID).Return(domain.RoleMember, nil)
	lister.On("ListComments", mock.Anything, taskID).Return([]domain.Comment(nil), errors.New("db"))

	_, err := newSvc(saver, lister, tasks, members).List(ctx, callerID, taskID)
	assert.Error(t, err)
}

func TestAdd_MemberCheckFails(t *testing.T) {
	saver := &mockSaver{}
	lister := &mockLister{}
	tasks := &mockTaskGetter{}
	members := &mockMemberChecker{}

	taskID, teamID, callerID := uuid.New(), uuid.New(), uuid.New()
	tasks.On("GetTaskByID", mock.Anything, taskID).Return(domain.Task{ID: taskID, TeamID: teamID}, nil)
	members.On("GetMemberRole", mock.Anything, teamID, callerID).Return(domain.Role(""), errors.New("db error"))

	_, err := newSvc(saver, lister, tasks, members).Add(ctx, callerID, taskID, "hello")
	assert.Error(t, err)
	assert.NotErrorIs(t, err, domain.ErrForbidden)
	saver.AssertNotCalled(t, "SaveComment")
}

func TestList_MemberCheckFails(t *testing.T) {
	saver := &mockSaver{}
	lister := &mockLister{}
	tasks := &mockTaskGetter{}
	members := &mockMemberChecker{}

	taskID, teamID, callerID := uuid.New(), uuid.New(), uuid.New()
	tasks.On("GetTaskByID", mock.Anything, taskID).Return(domain.Task{ID: taskID, TeamID: teamID}, nil)
	members.On("GetMemberRole", mock.Anything, teamID, callerID).Return(domain.Role(""), errors.New("db error"))

	_, err := newSvc(saver, lister, tasks, members).List(ctx, callerID, taskID)
	assert.Error(t, err)
	assert.NotErrorIs(t, err, domain.ErrForbidden)
}
