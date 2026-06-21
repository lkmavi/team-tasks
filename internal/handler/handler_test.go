package handler_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	openapiTypes "github.com/oapi-codegen/runtime/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/lkmavi/team-tasks/internal/domain"
	"github.com/lkmavi/team-tasks/internal/handler"
	v1 "github.com/lkmavi/team-tasks/internal/handler/api/v1"
	"github.com/lkmavi/team-tasks/internal/middleware"
)

// mock services

type mockAuthSvc struct{ mock.Mock }

func (m *mockAuthSvc) Register(ctx context.Context, email, name, password string) error {
	return m.Called(ctx, email, name, password).Error(0)
}
func (m *mockAuthSvc) Login(ctx context.Context, email, password string) (string, error) {
	args := m.Called(ctx, email, password)
	return args.String(0), args.Error(1)
}

type mockTeamSvc struct{ mock.Mock }

func (m *mockTeamSvc) Create(ctx context.Context, ownerID uuid.UUID, name string) (domain.Team, error) {
	args := m.Called(ctx, ownerID, name)
	return args.Get(0).(domain.Team), args.Error(1)
}
func (m *mockTeamSvc) ListForUser(ctx context.Context, userID uuid.UUID) ([]domain.Team, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).([]domain.Team), args.Error(1)
}
func (m *mockTeamSvc) Invite(ctx context.Context, callerID, teamID, targetUserID uuid.UUID) error {
	return m.Called(ctx, callerID, teamID, targetUserID).Error(0)
}

type mockTaskSvc struct{ mock.Mock }

func (m *mockTaskSvc) Create(ctx context.Context, creatorID uuid.UUID, input domain.CreateTaskInput) (domain.Task, error) {
	args := m.Called(ctx, creatorID, input)
	return args.Get(0).(domain.Task), args.Error(1)
}
func (m *mockTaskSvc) List(ctx context.Context, callerID uuid.UUID, filter domain.TaskFilter) ([]domain.Task, int, error) {
	args := m.Called(ctx, callerID, filter)
	return args.Get(0).([]domain.Task), args.Int(1), args.Error(2)
}
func (m *mockTaskSvc) Update(ctx context.Context, callerID, taskID uuid.UUID, input domain.UpdateTaskInput) (domain.Task, error) {
	args := m.Called(ctx, callerID, taskID, input)
	return args.Get(0).(domain.Task), args.Error(1)
}
func (m *mockTaskSvc) History(ctx context.Context, callerID, taskID uuid.UUID) ([]domain.TaskHistory, error) {
	args := m.Called(ctx, callerID, taskID)
	return args.Get(0).([]domain.TaskHistory), args.Error(1)
}

type mockCommentSvc struct{ mock.Mock }

func (m *mockCommentSvc) Add(ctx context.Context, callerID, taskID uuid.UUID, body string) (domain.Comment, error) {
	args := m.Called(ctx, callerID, taskID, body)
	return args.Get(0).(domain.Comment), args.Error(1)
}
func (m *mockCommentSvc) List(ctx context.Context, callerID, taskID uuid.UUID) ([]domain.Comment, error) {
	args := m.Called(ctx, callerID, taskID)
	return args.Get(0).([]domain.Comment), args.Error(1)
}

type mockAnalyticsSvc struct{ mock.Mock }

func (m *mockAnalyticsSvc) TeamSummaries(ctx context.Context) ([]domain.TeamSummary, error) {
	args := m.Called(ctx)
	return args.Get(0).([]domain.TeamSummary), args.Error(1)
}
func (m *mockAnalyticsSvc) TopContributors(ctx context.Context) ([]domain.TopContributor, error) {
	args := m.Called(ctx)
	return args.Get(0).([]domain.TopContributor), args.Error(1)
}
func (m *mockAnalyticsSvc) OrphanTasks(ctx context.Context) ([]domain.OrphanTask, error) {
	args := m.Called(ctx)
	return args.Get(0).([]domain.OrphanTask), args.Error(1)
}

// helpers

var (
	bgCtx  = context.Background()
	authID = uuid.New()
	authed = middleware.ContextWithUserID(bgCtx, authID)
)

func newHandler(auth *mockAuthSvc, teams *mockTeamSvc, tasks *mockTaskSvc, comments *mockCommentSvc, analytics *mockAnalyticsSvc) *handler.Handler {
	return handler.New(auth, teams, tasks, comments, analytics)
}

var errInternal = errors.New("internal error")

// auth handler tests

func TestRegister_NilBody(t *testing.T) {
	h := newHandler(&mockAuthSvc{}, &mockTeamSvc{}, &mockTaskSvc{}, &mockCommentSvc{}, &mockAnalyticsSvc{})
	resp, err := h.Register(bgCtx, v1.RegisterRequestObject{Body: nil})
	require.NoError(t, err)
	assert.IsType(t, v1.Register400JSONResponse{}, resp)
}

func TestRegister_InternalError(t *testing.T) {
	auth := &mockAuthSvc{}
	auth.On("Register", mock.Anything, "a@b.com", "Name", "pass").Return(errInternal)
	h := newHandler(auth, &mockTeamSvc{}, &mockTaskSvc{}, &mockCommentSvc{}, &mockAnalyticsSvc{})
	_, err := h.Register(bgCtx, v1.RegisterRequestObject{Body: &v1.RegisterRequest{
		Email: "a@b.com", Name: "Name", Password: "pass",
	}})
	assert.Error(t, err)
}

func TestLogin_NilBody(t *testing.T) {
	h := newHandler(&mockAuthSvc{}, &mockTeamSvc{}, &mockTaskSvc{}, &mockCommentSvc{}, &mockAnalyticsSvc{})
	resp, err := h.Login(bgCtx, v1.LoginRequestObject{Body: nil})
	require.NoError(t, err)
	assert.IsType(t, v1.Login400JSONResponse{}, resp)
}

func TestLogin_InternalError(t *testing.T) {
	auth := &mockAuthSvc{}
	auth.On("Login", mock.Anything, "a@b.com", "pass").Return("", errInternal)
	h := newHandler(auth, &mockTeamSvc{}, &mockTaskSvc{}, &mockCommentSvc{}, &mockAnalyticsSvc{})
	_, err := h.Login(bgCtx, v1.LoginRequestObject{Body: &v1.LoginRequest{Email: "a@b.com", Password: "pass"}})
	assert.Error(t, err)
}

// team handler tests

func TestListTeams_InternalError(t *testing.T) {
	teams := &mockTeamSvc{}
	teams.On("ListForUser", mock.Anything, authID).Return([]domain.Team(nil), errInternal)
	h := newHandler(&mockAuthSvc{}, teams, &mockTaskSvc{}, &mockCommentSvc{}, &mockAnalyticsSvc{})
	_, err := h.ListTeams(authed, v1.ListTeamsRequestObject{})
	assert.Error(t, err)
}

func TestCreateTeam_NilBody(t *testing.T) {
	h := newHandler(&mockAuthSvc{}, &mockTeamSvc{}, &mockTaskSvc{}, &mockCommentSvc{}, &mockAnalyticsSvc{})
	resp, err := h.CreateTeam(authed, v1.CreateTeamRequestObject{Body: nil})
	require.NoError(t, err)
	assert.IsType(t, v1.CreateTeam400JSONResponse{}, resp)
}

func TestCreateTeam_InternalError(t *testing.T) {
	teams := &mockTeamSvc{}
	teams.On("Create", mock.Anything, authID, "TeamX").Return(domain.Team{}, errInternal)
	h := newHandler(&mockAuthSvc{}, teams, &mockTaskSvc{}, &mockCommentSvc{}, &mockAnalyticsSvc{})
	_, err := h.CreateTeam(authed, v1.CreateTeamRequestObject{Body: &v1.CreateTeamRequest{Name: "TeamX"}})
	assert.Error(t, err)
}

func TestInviteToTeam_NilBody(t *testing.T) {
	h := newHandler(&mockAuthSvc{}, &mockTeamSvc{}, &mockTaskSvc{}, &mockCommentSvc{}, &mockAnalyticsSvc{})
	resp, err := h.InviteToTeam(authed, v1.InviteToTeamRequestObject{Id: uuid.New(), Body: nil})
	require.NoError(t, err)
	assert.IsType(t, v1.InviteToTeam400JSONResponse{}, resp)
}

func TestInviteToTeam_InternalError(t *testing.T) {
	teams := &mockTeamSvc{}
	teamID, targetID := uuid.New(), uuid.New()
	teams.On("Invite", mock.Anything, authID, teamID, targetID).Return(errInternal)
	h := newHandler(&mockAuthSvc{}, teams, &mockTaskSvc{}, &mockCommentSvc{}, &mockAnalyticsSvc{})
	_, err := h.InviteToTeam(authed, v1.InviteToTeamRequestObject{
		Id:   teamID,
		Body: &v1.InviteRequest{UserId: targetID},
	})
	assert.Error(t, err)
}

// task handler tests

func TestCreateTask_NilBody(t *testing.T) {
	h := newHandler(&mockAuthSvc{}, &mockTeamSvc{}, &mockTaskSvc{}, &mockCommentSvc{}, &mockAnalyticsSvc{})
	resp, err := h.CreateTask(authed, v1.CreateTaskRequestObject{Body: nil})
	require.NoError(t, err)
	assert.IsType(t, v1.CreateTask400JSONResponse{}, resp)
}

func TestCreateTask_InternalError(t *testing.T) {
	tasks := &mockTaskSvc{}
	tasks.On("Create", mock.Anything, authID, mock.AnythingOfType("domain.CreateTaskInput")).Return(domain.Task{}, errInternal)
	h := newHandler(&mockAuthSvc{}, &mockTeamSvc{}, tasks, &mockCommentSvc{}, &mockAnalyticsSvc{})
	_, err := h.CreateTask(authed, v1.CreateTaskRequestObject{Body: &v1.CreateTaskRequest{Title: "x"}})
	assert.Error(t, err)
}

func TestListTasks_InternalError(t *testing.T) {
	tasks := &mockTaskSvc{}
	tasks.On("List", mock.Anything, authID, mock.AnythingOfType("domain.TaskFilter")).Return([]domain.Task(nil), 0, errInternal)
	h := newHandler(&mockAuthSvc{}, &mockTeamSvc{}, tasks, &mockCommentSvc{}, &mockAnalyticsSvc{})
	_, err := h.ListTasks(authed, v1.ListTasksRequestObject{})
	assert.Error(t, err)
}

func TestUpdateTask_NilBody(t *testing.T) {
	h := newHandler(&mockAuthSvc{}, &mockTeamSvc{}, &mockTaskSvc{}, &mockCommentSvc{}, &mockAnalyticsSvc{})
	resp, err := h.UpdateTask(authed, v1.UpdateTaskRequestObject{Id: uuid.New(), Body: nil})
	require.NoError(t, err)
	assert.IsType(t, v1.UpdateTask400JSONResponse{}, resp)
}

func TestUpdateTask_InternalError(t *testing.T) {
	tasks := &mockTaskSvc{}
	taskID := uuid.New()
	tasks.On("Update", mock.Anything, authID, taskID, mock.AnythingOfType("domain.UpdateTaskInput")).Return(domain.Task{}, errInternal)
	h := newHandler(&mockAuthSvc{}, &mockTeamSvc{}, tasks, &mockCommentSvc{}, &mockAnalyticsSvc{})
	title := "x"
	_, err := h.UpdateTask(authed, v1.UpdateTaskRequestObject{Id: taskID, Body: &v1.UpdateTaskRequest{Title: &title}})
	assert.Error(t, err)
}

func TestGetTaskHistory_InternalError(t *testing.T) {
	tasks := &mockTaskSvc{}
	taskID := uuid.New()
	tasks.On("History", mock.Anything, authID, taskID).Return([]domain.TaskHistory(nil), errInternal)
	h := newHandler(&mockAuthSvc{}, &mockTeamSvc{}, tasks, &mockCommentSvc{}, &mockAnalyticsSvc{})
	_, err := h.GetTaskHistory(authed, v1.GetTaskHistoryRequestObject{Id: taskID})
	assert.Error(t, err)
}

// comment handler tests

func TestAddTaskComment_NilBody(t *testing.T) {
	h := newHandler(&mockAuthSvc{}, &mockTeamSvc{}, &mockTaskSvc{}, &mockCommentSvc{}, &mockAnalyticsSvc{})
	resp, err := h.AddTaskComment(authed, v1.AddTaskCommentRequestObject{Id: uuid.New(), Body: nil})
	require.NoError(t, err)
	assert.IsType(t, v1.AddTaskComment400JSONResponse{}, resp)
}

func TestAddTaskComment_EmptyBody(t *testing.T) {
	h := newHandler(&mockAuthSvc{}, &mockTeamSvc{}, &mockTaskSvc{}, &mockCommentSvc{}, &mockAnalyticsSvc{})
	resp, err := h.AddTaskComment(authed, v1.AddTaskCommentRequestObject{Id: uuid.New(), Body: &v1.AddCommentRequest{Body: ""}})
	require.NoError(t, err)
	assert.IsType(t, v1.AddTaskComment400JSONResponse{}, resp)
}

func TestAddTaskComment_InternalError(t *testing.T) {
	comments := &mockCommentSvc{}
	taskID := uuid.New()
	comments.On("Add", mock.Anything, authID, taskID, "hello").Return(domain.Comment{}, errInternal)
	h := newHandler(&mockAuthSvc{}, &mockTeamSvc{}, &mockTaskSvc{}, comments, &mockAnalyticsSvc{})
	_, err := h.AddTaskComment(authed, v1.AddTaskCommentRequestObject{Id: taskID, Body: &v1.AddCommentRequest{Body: "hello"}})
	assert.Error(t, err)
}

func TestListTaskComments_InternalError(t *testing.T) {
	comments := &mockCommentSvc{}
	taskID := uuid.New()
	comments.On("List", mock.Anything, authID, taskID).Return([]domain.Comment(nil), errInternal)
	h := newHandler(&mockAuthSvc{}, &mockTeamSvc{}, &mockTaskSvc{}, comments, &mockAnalyticsSvc{})
	_, err := h.ListTaskComments(authed, v1.ListTaskCommentsRequestObject{Id: taskID})
	assert.Error(t, err)
}

// analytics handler tests

func TestGetTeamSummaries_InternalError(t *testing.T) {
	analytics := &mockAnalyticsSvc{}
	analytics.On("TeamSummaries", mock.Anything).Return([]domain.TeamSummary(nil), errInternal)
	h := newHandler(&mockAuthSvc{}, &mockTeamSvc{}, &mockTaskSvc{}, &mockCommentSvc{}, analytics)
	_, err := h.GetTeamSummaries(authed, v1.GetTeamSummariesRequestObject{})
	assert.Error(t, err)
}

func TestGetTopContributors_InternalError(t *testing.T) {
	analytics := &mockAnalyticsSvc{}
	analytics.On("TopContributors", mock.Anything).Return([]domain.TopContributor(nil), errInternal)
	h := newHandler(&mockAuthSvc{}, &mockTeamSvc{}, &mockTaskSvc{}, &mockCommentSvc{}, analytics)
	_, err := h.GetTopContributors(authed, v1.GetTopContributorsRequestObject{})
	assert.Error(t, err)
}

func TestGetOrphanTasks_InternalError(t *testing.T) {
	analytics := &mockAnalyticsSvc{}
	analytics.On("OrphanTasks", mock.Anything).Return([]domain.OrphanTask(nil), errInternal)
	h := newHandler(&mockAuthSvc{}, &mockTeamSvc{}, &mockTaskSvc{}, &mockCommentSvc{}, analytics)
	_, err := h.GetOrphanTasks(authed, v1.GetOrphanTasksRequestObject{})
	assert.Error(t, err)
}

// ── auth success + additional error paths ─────────────────────────────────────

func TestRegister_OK(t *testing.T) {
	auth := &mockAuthSvc{}
	auth.On("Register", mock.Anything, "a@b.com", "Name", "pass").Return(nil)
	h := newHandler(auth, &mockTeamSvc{}, &mockTaskSvc{}, &mockCommentSvc{}, &mockAnalyticsSvc{})
	resp, err := h.Register(bgCtx, v1.RegisterRequestObject{Body: &v1.RegisterRequest{
		Email: "a@b.com", Name: "Name", Password: "pass",
	}})
	require.NoError(t, err)
	assert.IsType(t, v1.Register201Response{}, resp)
}

func TestRegister_Conflict(t *testing.T) {
	auth := &mockAuthSvc{}
	auth.On("Register", mock.Anything, "a@b.com", "Name", "pass").Return(domain.ErrConflict)
	h := newHandler(auth, &mockTeamSvc{}, &mockTaskSvc{}, &mockCommentSvc{}, &mockAnalyticsSvc{})
	resp, err := h.Register(bgCtx, v1.RegisterRequestObject{Body: &v1.RegisterRequest{
		Email: "a@b.com", Name: "Name", Password: "pass",
	}})
	require.NoError(t, err)
	assert.IsType(t, v1.Register409JSONResponse{}, resp)
}

func TestLogin_OK(t *testing.T) {
	auth := &mockAuthSvc{}
	auth.On("Login", mock.Anything, "a@b.com", "pass").Return("tok", nil)
	h := newHandler(auth, &mockTeamSvc{}, &mockTaskSvc{}, &mockCommentSvc{}, &mockAnalyticsSvc{})
	resp, err := h.Login(bgCtx, v1.LoginRequestObject{Body: &v1.LoginRequest{Email: "a@b.com", Password: "pass"}})
	require.NoError(t, err)
	assert.Equal(t, v1.Login200JSONResponse{Token: "tok"}, resp)
}

func TestLogin_InvalidCredentials(t *testing.T) {
	auth := &mockAuthSvc{}
	auth.On("Login", mock.Anything, "a@b.com", "wrong").Return("", domain.ErrUnauthorized)
	h := newHandler(auth, &mockTeamSvc{}, &mockTaskSvc{}, &mockCommentSvc{}, &mockAnalyticsSvc{})
	resp, err := h.Login(bgCtx, v1.LoginRequestObject{Body: &v1.LoginRequest{Email: "a@b.com", Password: "wrong"}})
	require.NoError(t, err)
	assert.IsType(t, v1.Login401JSONResponse{}, resp)
}

// ── team success + unauthorized paths ─────────────────────────────────────────

func TestListTeams_OK(t *testing.T) {
	teams := &mockTeamSvc{}
	teams.On("ListForUser", mock.Anything, authID).Return([]domain.Team{{ID: uuid.New(), Name: "T1"}}, nil)
	h := newHandler(&mockAuthSvc{}, teams, &mockTaskSvc{}, &mockCommentSvc{}, &mockAnalyticsSvc{})
	resp, err := h.ListTeams(authed, v1.ListTeamsRequestObject{})
	require.NoError(t, err)
	assert.IsType(t, v1.ListTeams200JSONResponse{}, resp)
}

func TestListTeams_Unauthorized(t *testing.T) {
	h := newHandler(&mockAuthSvc{}, &mockTeamSvc{}, &mockTaskSvc{}, &mockCommentSvc{}, &mockAnalyticsSvc{})
	resp, err := h.ListTeams(bgCtx, v1.ListTeamsRequestObject{})
	require.NoError(t, err)
	assert.IsType(t, v1.ListTeams401JSONResponse{}, resp)
}

func TestCreateTeam_OK(t *testing.T) {
	teams := &mockTeamSvc{}
	teams.On("Create", mock.Anything, authID, "T1").Return(domain.Team{ID: uuid.New(), Name: "T1"}, nil)
	h := newHandler(&mockAuthSvc{}, teams, &mockTaskSvc{}, &mockCommentSvc{}, &mockAnalyticsSvc{})
	resp, err := h.CreateTeam(authed, v1.CreateTeamRequestObject{Body: &v1.CreateTeamRequest{Name: "T1"}})
	require.NoError(t, err)
	assert.IsType(t, v1.CreateTeam201JSONResponse{}, resp)
}

func TestCreateTeam_Unauthorized(t *testing.T) {
	h := newHandler(&mockAuthSvc{}, &mockTeamSvc{}, &mockTaskSvc{}, &mockCommentSvc{}, &mockAnalyticsSvc{})
	resp, err := h.CreateTeam(bgCtx, v1.CreateTeamRequestObject{Body: &v1.CreateTeamRequest{Name: "T1"}})
	require.NoError(t, err)
	assert.IsType(t, v1.CreateTeam401JSONResponse{}, resp)
}

func TestInviteToTeam_OK(t *testing.T) {
	teams := &mockTeamSvc{}
	teamID, targetID := uuid.New(), uuid.New()
	teams.On("Invite", mock.Anything, authID, teamID, targetID).Return(nil)
	h := newHandler(&mockAuthSvc{}, teams, &mockTaskSvc{}, &mockCommentSvc{}, &mockAnalyticsSvc{})
	resp, err := h.InviteToTeam(authed, v1.InviteToTeamRequestObject{
		Id:   teamID,
		Body: &v1.InviteRequest{UserId: targetID},
	})
	require.NoError(t, err)
	assert.IsType(t, v1.InviteToTeam200Response{}, resp)
}

func TestInviteToTeam_Unauthorized(t *testing.T) {
	h := newHandler(&mockAuthSvc{}, &mockTeamSvc{}, &mockTaskSvc{}, &mockCommentSvc{}, &mockAnalyticsSvc{})
	resp, err := h.InviteToTeam(bgCtx, v1.InviteToTeamRequestObject{
		Id:   uuid.New(),
		Body: &v1.InviteRequest{UserId: uuid.New()},
	})
	require.NoError(t, err)
	assert.IsType(t, v1.InviteToTeam401JSONResponse{}, resp)
}

func TestInviteToTeam_Forbidden(t *testing.T) {
	teams := &mockTeamSvc{}
	teamID, targetID := uuid.New(), uuid.New()
	teams.On("Invite", mock.Anything, authID, teamID, targetID).Return(domain.ErrForbidden)
	h := newHandler(&mockAuthSvc{}, teams, &mockTaskSvc{}, &mockCommentSvc{}, &mockAnalyticsSvc{})
	resp, err := h.InviteToTeam(authed, v1.InviteToTeamRequestObject{
		Id:   teamID,
		Body: &v1.InviteRequest{UserId: targetID},
	})
	require.NoError(t, err)
	assert.IsType(t, v1.InviteToTeam403JSONResponse{}, resp)
}

func TestInviteToTeam_NotFound(t *testing.T) {
	teams := &mockTeamSvc{}
	teamID, targetID := uuid.New(), uuid.New()
	teams.On("Invite", mock.Anything, authID, teamID, targetID).Return(domain.ErrNotFound)
	h := newHandler(&mockAuthSvc{}, teams, &mockTaskSvc{}, &mockCommentSvc{}, &mockAnalyticsSvc{})
	resp, err := h.InviteToTeam(authed, v1.InviteToTeamRequestObject{
		Id:   teamID,
		Body: &v1.InviteRequest{UserId: targetID},
	})
	require.NoError(t, err)
	assert.IsType(t, v1.InviteToTeam404JSONResponse{}, resp)
}

// ── task success + unauthorized + additional error paths ──────────────────────

func TestCreateTask_OK(t *testing.T) {
	tasks := &mockTaskSvc{}
	tasks.On("Create", mock.Anything, authID, mock.AnythingOfType("domain.CreateTaskInput")).
		Return(domain.Task{ID: uuid.New(), Title: "x"}, nil)
	h := newHandler(&mockAuthSvc{}, &mockTeamSvc{}, tasks, &mockCommentSvc{}, &mockAnalyticsSvc{})
	resp, err := h.CreateTask(authed, v1.CreateTaskRequestObject{Body: &v1.CreateTaskRequest{Title: "x"}})
	require.NoError(t, err)
	assert.IsType(t, v1.CreateTask201JSONResponse{}, resp)
}

func TestCreateTask_Unauthorized(t *testing.T) {
	h := newHandler(&mockAuthSvc{}, &mockTeamSvc{}, &mockTaskSvc{}, &mockCommentSvc{}, &mockAnalyticsSvc{})
	resp, err := h.CreateTask(bgCtx, v1.CreateTaskRequestObject{Body: &v1.CreateTaskRequest{Title: "x"}})
	require.NoError(t, err)
	assert.IsType(t, v1.CreateTask401JSONResponse{}, resp)
}

func TestCreateTask_Forbidden(t *testing.T) {
	tasks := &mockTaskSvc{}
	tasks.On("Create", mock.Anything, authID, mock.AnythingOfType("domain.CreateTaskInput")).
		Return(domain.Task{}, domain.ErrForbidden)
	h := newHandler(&mockAuthSvc{}, &mockTeamSvc{}, tasks, &mockCommentSvc{}, &mockAnalyticsSvc{})
	resp, err := h.CreateTask(authed, v1.CreateTaskRequestObject{Body: &v1.CreateTaskRequest{Title: "x"}})
	require.NoError(t, err)
	assert.IsType(t, v1.CreateTask403JSONResponse{}, resp)
}

// TestCreateTask_WithOptionalFields covers AssigneeId/Priority/DueDate branches in the
// handler and the AssigneeID/DueDate branches in toAPITask.
func TestCreateTask_WithOptionalFields(t *testing.T) {
	tasks := &mockTaskSvc{}
	assigneeID := uuid.New()
	dueTime := time.Now()
	returned := domain.Task{
		ID:         uuid.New(),
		Title:      "T",
		AssigneeID: &assigneeID,
		DueDate:    &dueTime,
	}
	tasks.On("Create", mock.Anything, authID, mock.AnythingOfType("domain.CreateTaskInput")).
		Return(returned, nil)
	h := newHandler(&mockAuthSvc{}, &mockTeamSvc{}, tasks, &mockCommentSvc{}, &mockAnalyticsSvc{})

	aid := assigneeID
	pri := v1.High
	due := openapiTypes.Date{Time: dueTime}
	resp, err := h.CreateTask(authed, v1.CreateTaskRequestObject{Body: &v1.CreateTaskRequest{
		Title:      "T",
		AssigneeId: &aid,
		Priority:   &pri,
		DueDate:    &due,
	}})
	require.NoError(t, err)
	assert.IsType(t, v1.CreateTask201JSONResponse{}, resp)
}

func TestListTasks_OK(t *testing.T) {
	tasks := &mockTaskSvc{}
	tasks.On("List", mock.Anything, authID, mock.AnythingOfType("domain.TaskFilter")).
		Return([]domain.Task{{ID: uuid.New(), Title: "x"}}, 1, nil)
	h := newHandler(&mockAuthSvc{}, &mockTeamSvc{}, tasks, &mockCommentSvc{}, &mockAnalyticsSvc{})
	resp, err := h.ListTasks(authed, v1.ListTasksRequestObject{})
	require.NoError(t, err)
	assert.IsType(t, v1.ListTasks200JSONResponse{}, resp)
}

func TestListTasks_Unauthorized(t *testing.T) {
	h := newHandler(&mockAuthSvc{}, &mockTeamSvc{}, &mockTaskSvc{}, &mockCommentSvc{}, &mockAnalyticsSvc{})
	resp, err := h.ListTasks(bgCtx, v1.ListTasksRequestObject{})
	require.NoError(t, err)
	assert.IsType(t, v1.ListTasks401JSONResponse{}, resp)
}

// TestListTasks_WithOptionalParams covers all optional filter/pagination branches.
func TestListTasks_WithOptionalParams(t *testing.T) {
	tasks := &mockTaskSvc{}
	tasks.On("List", mock.Anything, authID, mock.AnythingOfType("domain.TaskFilter")).
		Return([]domain.Task{}, 0, nil)
	h := newHandler(&mockAuthSvc{}, &mockTeamSvc{}, tasks, &mockCommentSvc{}, &mockAnalyticsSvc{})

	teamID := uuid.New()
	status := v1.Todo
	assigneeID := uuid.New()
	page, limit := 2, 20
	resp, err := h.ListTasks(authed, v1.ListTasksRequestObject{
		Params: v1.ListTasksParams{
			TeamId:     &teamID,
			Status:     &status,
			AssigneeId: &assigneeID,
			Page:       &page,
			Limit:      &limit,
		},
	})
	require.NoError(t, err)
	assert.IsType(t, v1.ListTasks200JSONResponse{}, resp)
}

func TestUpdateTask_OK(t *testing.T) {
	tasks := &mockTaskSvc{}
	taskID := uuid.New()
	tasks.On("Update", mock.Anything, authID, taskID, mock.AnythingOfType("domain.UpdateTaskInput")).
		Return(domain.Task{ID: taskID, Title: "updated"}, nil)
	h := newHandler(&mockAuthSvc{}, &mockTeamSvc{}, tasks, &mockCommentSvc{}, &mockAnalyticsSvc{})
	title := "updated"
	resp, err := h.UpdateTask(authed, v1.UpdateTaskRequestObject{
		Id:   taskID,
		Body: &v1.UpdateTaskRequest{Title: &title},
	})
	require.NoError(t, err)
	assert.IsType(t, v1.UpdateTask200JSONResponse{}, resp)
}

func TestUpdateTask_Unauthorized(t *testing.T) {
	h := newHandler(&mockAuthSvc{}, &mockTeamSvc{}, &mockTaskSvc{}, &mockCommentSvc{}, &mockAnalyticsSvc{})
	title := "x"
	resp, err := h.UpdateTask(bgCtx, v1.UpdateTaskRequestObject{
		Id:   uuid.New(),
		Body: &v1.UpdateTaskRequest{Title: &title},
	})
	require.NoError(t, err)
	assert.IsType(t, v1.UpdateTask401JSONResponse{}, resp)
}

func TestUpdateTask_NotFound(t *testing.T) {
	tasks := &mockTaskSvc{}
	taskID := uuid.New()
	tasks.On("Update", mock.Anything, authID, taskID, mock.AnythingOfType("domain.UpdateTaskInput")).
		Return(domain.Task{}, domain.ErrNotFound)
	h := newHandler(&mockAuthSvc{}, &mockTeamSvc{}, tasks, &mockCommentSvc{}, &mockAnalyticsSvc{})
	title := "x"
	resp, err := h.UpdateTask(authed, v1.UpdateTaskRequestObject{
		Id:   taskID,
		Body: &v1.UpdateTaskRequest{Title: &title},
	})
	require.NoError(t, err)
	assert.IsType(t, v1.UpdateTask404JSONResponse{}, resp)
}

func TestUpdateTask_Forbidden(t *testing.T) {
	tasks := &mockTaskSvc{}
	taskID := uuid.New()
	tasks.On("Update", mock.Anything, authID, taskID, mock.AnythingOfType("domain.UpdateTaskInput")).
		Return(domain.Task{}, domain.ErrForbidden)
	h := newHandler(&mockAuthSvc{}, &mockTeamSvc{}, tasks, &mockCommentSvc{}, &mockAnalyticsSvc{})
	title := "x"
	resp, err := h.UpdateTask(authed, v1.UpdateTaskRequestObject{
		Id:   taskID,
		Body: &v1.UpdateTaskRequest{Title: &title},
	})
	require.NoError(t, err)
	assert.IsType(t, v1.UpdateTask403JSONResponse{}, resp)
}

// TestUpdateTask_WithOptionalFields covers AssigneeId/Status/Priority/DueDate branches.
func TestUpdateTask_WithOptionalFields(t *testing.T) {
	tasks := &mockTaskSvc{}
	taskID, assigneeID := uuid.New(), uuid.New()
	dueTime := time.Now()
	tasks.On("Update", mock.Anything, authID, taskID, mock.AnythingOfType("domain.UpdateTaskInput")).
		Return(domain.Task{ID: taskID, AssigneeID: &assigneeID, DueDate: &dueTime}, nil)
	h := newHandler(&mockAuthSvc{}, &mockTeamSvc{}, tasks, &mockCommentSvc{}, &mockAnalyticsSvc{})

	aid := assigneeID
	status := v1.InProgress
	pri := v1.High
	due := openapiTypes.Date{Time: dueTime}
	resp, err := h.UpdateTask(authed, v1.UpdateTaskRequestObject{
		Id: taskID,
		Body: &v1.UpdateTaskRequest{
			AssigneeId: &aid,
			Status:     &status,
			Priority:   &pri,
			DueDate:    &due,
		},
	})
	require.NoError(t, err)
	assert.IsType(t, v1.UpdateTask200JSONResponse{}, resp)
}

func TestGetTaskHistory_OK(t *testing.T) {
	tasks := &mockTaskSvc{}
	taskID := uuid.New()
	tasks.On("History", mock.Anything, authID, taskID).
		Return([]domain.TaskHistory{{ID: 1, Field: "title"}}, nil)
	h := newHandler(&mockAuthSvc{}, &mockTeamSvc{}, tasks, &mockCommentSvc{}, &mockAnalyticsSvc{})
	resp, err := h.GetTaskHistory(authed, v1.GetTaskHistoryRequestObject{Id: taskID})
	require.NoError(t, err)
	assert.IsType(t, v1.GetTaskHistory200JSONResponse{}, resp)
}

func TestGetTaskHistory_Unauthorized(t *testing.T) {
	h := newHandler(&mockAuthSvc{}, &mockTeamSvc{}, &mockTaskSvc{}, &mockCommentSvc{}, &mockAnalyticsSvc{})
	resp, err := h.GetTaskHistory(bgCtx, v1.GetTaskHistoryRequestObject{Id: uuid.New()})
	require.NoError(t, err)
	assert.IsType(t, v1.GetTaskHistory401JSONResponse{}, resp)
}

func TestGetTaskHistory_NotFound(t *testing.T) {
	tasks := &mockTaskSvc{}
	taskID := uuid.New()
	tasks.On("History", mock.Anything, authID, taskID).
		Return([]domain.TaskHistory(nil), domain.ErrNotFound)
	h := newHandler(&mockAuthSvc{}, &mockTeamSvc{}, tasks, &mockCommentSvc{}, &mockAnalyticsSvc{})
	resp, err := h.GetTaskHistory(authed, v1.GetTaskHistoryRequestObject{Id: taskID})
	require.NoError(t, err)
	assert.IsType(t, v1.GetTaskHistory404JSONResponse{}, resp)
}

func TestGetTaskHistory_Forbidden(t *testing.T) {
	tasks := &mockTaskSvc{}
	taskID := uuid.New()
	tasks.On("History", mock.Anything, authID, taskID).
		Return([]domain.TaskHistory(nil), domain.ErrForbidden)
	h := newHandler(&mockAuthSvc{}, &mockTeamSvc{}, tasks, &mockCommentSvc{}, &mockAnalyticsSvc{})
	resp, err := h.GetTaskHistory(authed, v1.GetTaskHistoryRequestObject{Id: taskID})
	require.NoError(t, err)
	assert.IsType(t, v1.GetTaskHistory403JSONResponse{}, resp)
}

// ── comment success + unauthorized + additional error paths ───────────────────

func TestAddTaskComment_OK(t *testing.T) {
	comments := &mockCommentSvc{}
	taskID := uuid.New()
	comments.On("Add", mock.Anything, authID, taskID, "hello").
		Return(domain.Comment{ID: 1, Body: "hello"}, nil)
	h := newHandler(&mockAuthSvc{}, &mockTeamSvc{}, &mockTaskSvc{}, comments, &mockAnalyticsSvc{})
	resp, err := h.AddTaskComment(authed, v1.AddTaskCommentRequestObject{
		Id:   taskID,
		Body: &v1.AddCommentRequest{Body: "hello"},
	})
	require.NoError(t, err)
	assert.IsType(t, v1.AddTaskComment201JSONResponse{}, resp)
}

func TestAddTaskComment_Unauthorized(t *testing.T) {
	h := newHandler(&mockAuthSvc{}, &mockTeamSvc{}, &mockTaskSvc{}, &mockCommentSvc{}, &mockAnalyticsSvc{})
	resp, err := h.AddTaskComment(bgCtx, v1.AddTaskCommentRequestObject{
		Id:   uuid.New(),
		Body: &v1.AddCommentRequest{Body: "hello"},
	})
	require.NoError(t, err)
	assert.IsType(t, v1.AddTaskComment401JSONResponse{}, resp)
}

func TestAddTaskComment_NotFound(t *testing.T) {
	comments := &mockCommentSvc{}
	taskID := uuid.New()
	comments.On("Add", mock.Anything, authID, taskID, "hello").
		Return(domain.Comment{}, domain.ErrNotFound)
	h := newHandler(&mockAuthSvc{}, &mockTeamSvc{}, &mockTaskSvc{}, comments, &mockAnalyticsSvc{})
	resp, err := h.AddTaskComment(authed, v1.AddTaskCommentRequestObject{
		Id:   taskID,
		Body: &v1.AddCommentRequest{Body: "hello"},
	})
	require.NoError(t, err)
	assert.IsType(t, v1.AddTaskComment404JSONResponse{}, resp)
}

func TestAddTaskComment_Forbidden(t *testing.T) {
	comments := &mockCommentSvc{}
	taskID := uuid.New()
	comments.On("Add", mock.Anything, authID, taskID, "hello").
		Return(domain.Comment{}, domain.ErrForbidden)
	h := newHandler(&mockAuthSvc{}, &mockTeamSvc{}, &mockTaskSvc{}, comments, &mockAnalyticsSvc{})
	resp, err := h.AddTaskComment(authed, v1.AddTaskCommentRequestObject{
		Id:   taskID,
		Body: &v1.AddCommentRequest{Body: "hello"},
	})
	require.NoError(t, err)
	assert.IsType(t, v1.AddTaskComment403JSONResponse{}, resp)
}

func TestListTaskComments_OK(t *testing.T) {
	comments := &mockCommentSvc{}
	taskID := uuid.New()
	comments.On("List", mock.Anything, authID, taskID).
		Return([]domain.Comment{{ID: 1, Body: "hi"}}, nil)
	h := newHandler(&mockAuthSvc{}, &mockTeamSvc{}, &mockTaskSvc{}, comments, &mockAnalyticsSvc{})
	resp, err := h.ListTaskComments(authed, v1.ListTaskCommentsRequestObject{Id: taskID})
	require.NoError(t, err)
	assert.IsType(t, v1.ListTaskComments200JSONResponse{}, resp)
}

func TestListTaskComments_Unauthorized(t *testing.T) {
	h := newHandler(&mockAuthSvc{}, &mockTeamSvc{}, &mockTaskSvc{}, &mockCommentSvc{}, &mockAnalyticsSvc{})
	resp, err := h.ListTaskComments(bgCtx, v1.ListTaskCommentsRequestObject{Id: uuid.New()})
	require.NoError(t, err)
	assert.IsType(t, v1.ListTaskComments401JSONResponse{}, resp)
}

func TestListTaskComments_NotFound(t *testing.T) {
	comments := &mockCommentSvc{}
	taskID := uuid.New()
	comments.On("List", mock.Anything, authID, taskID).
		Return([]domain.Comment(nil), domain.ErrNotFound)
	h := newHandler(&mockAuthSvc{}, &mockTeamSvc{}, &mockTaskSvc{}, comments, &mockAnalyticsSvc{})
	resp, err := h.ListTaskComments(authed, v1.ListTaskCommentsRequestObject{Id: taskID})
	require.NoError(t, err)
	assert.IsType(t, v1.ListTaskComments404JSONResponse{}, resp)
}

func TestListTaskComments_Forbidden(t *testing.T) {
	comments := &mockCommentSvc{}
	taskID := uuid.New()
	comments.On("List", mock.Anything, authID, taskID).
		Return([]domain.Comment(nil), domain.ErrForbidden)
	h := newHandler(&mockAuthSvc{}, &mockTeamSvc{}, &mockTaskSvc{}, comments, &mockAnalyticsSvc{})
	resp, err := h.ListTaskComments(authed, v1.ListTaskCommentsRequestObject{Id: taskID})
	require.NoError(t, err)
	assert.IsType(t, v1.ListTaskComments403JSONResponse{}, resp)
}

// ── analytics success + unauthorized paths ────────────────────────────────────

func TestGetTeamSummaries_OK(t *testing.T) {
	analytics := &mockAnalyticsSvc{}
	analytics.On("TeamSummaries", mock.Anything).
		Return([]domain.TeamSummary{{TeamID: uuid.New(), Name: "T1"}}, nil)
	h := newHandler(&mockAuthSvc{}, &mockTeamSvc{}, &mockTaskSvc{}, &mockCommentSvc{}, analytics)
	resp, err := h.GetTeamSummaries(authed, v1.GetTeamSummariesRequestObject{})
	require.NoError(t, err)
	assert.IsType(t, v1.GetTeamSummaries200JSONResponse{}, resp)
}

func TestGetTeamSummaries_Unauthorized(t *testing.T) {
	h := newHandler(&mockAuthSvc{}, &mockTeamSvc{}, &mockTaskSvc{}, &mockCommentSvc{}, &mockAnalyticsSvc{})
	resp, err := h.GetTeamSummaries(bgCtx, v1.GetTeamSummariesRequestObject{})
	require.NoError(t, err)
	assert.IsType(t, v1.GetTeamSummaries401JSONResponse{}, resp)
}

func TestGetTopContributors_OK(t *testing.T) {
	analytics := &mockAnalyticsSvc{}
	analytics.On("TopContributors", mock.Anything).
		Return([]domain.TopContributor{{TeamID: uuid.New(), UserID: uuid.New(), RankInTeam: 1}}, nil)
	h := newHandler(&mockAuthSvc{}, &mockTeamSvc{}, &mockTaskSvc{}, &mockCommentSvc{}, analytics)
	resp, err := h.GetTopContributors(authed, v1.GetTopContributorsRequestObject{})
	require.NoError(t, err)
	assert.IsType(t, v1.GetTopContributors200JSONResponse{}, resp)
}

func TestGetTopContributors_Unauthorized(t *testing.T) {
	h := newHandler(&mockAuthSvc{}, &mockTeamSvc{}, &mockTaskSvc{}, &mockCommentSvc{}, &mockAnalyticsSvc{})
	resp, err := h.GetTopContributors(bgCtx, v1.GetTopContributorsRequestObject{})
	require.NoError(t, err)
	assert.IsType(t, v1.GetTopContributors401JSONResponse{}, resp)
}

func TestGetOrphanTasks_OK(t *testing.T) {
	analytics := &mockAnalyticsSvc{}
	analytics.On("OrphanTasks", mock.Anything).
		Return([]domain.OrphanTask{{ID: uuid.New(), Title: "orphan"}}, nil)
	h := newHandler(&mockAuthSvc{}, &mockTeamSvc{}, &mockTaskSvc{}, &mockCommentSvc{}, analytics)
	resp, err := h.GetOrphanTasks(authed, v1.GetOrphanTasksRequestObject{})
	require.NoError(t, err)
	assert.IsType(t, v1.GetOrphanTasks200JSONResponse{}, resp)
}

func TestGetOrphanTasks_Unauthorized(t *testing.T) {
	h := newHandler(&mockAuthSvc{}, &mockTeamSvc{}, &mockTaskSvc{}, &mockCommentSvc{}, &mockAnalyticsSvc{})
	resp, err := h.GetOrphanTasks(bgCtx, v1.GetOrphanTasksRequestObject{})
	require.NoError(t, err)
	assert.IsType(t, v1.GetOrphanTasks401JSONResponse{}, resp)
}
