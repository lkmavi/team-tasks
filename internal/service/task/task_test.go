package task_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/lkmavi/team-tasks/internal/domain"
	. "github.com/lkmavi/team-tasks/internal/service/task"
)

type mockTaskStorage struct{ mock.Mock }

func (m *mockTaskStorage) SaveTask(ctx context.Context, task domain.Task) error {
	return m.Called(ctx, task).Error(0)
}
func (m *mockTaskStorage) GetTaskByID(ctx context.Context, id uuid.UUID) (domain.Task, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(domain.Task), args.Error(1)
}
func (m *mockTaskStorage) ListTasks(ctx context.Context, filter domain.TaskFilter) ([]domain.Task, int, error) {
	args := m.Called(ctx, filter)
	return args.Get(0).([]domain.Task), args.Int(1), args.Error(2)
}
func (m *mockTaskStorage) UpdateTaskWithHistory(ctx context.Context, task domain.Task, entries []domain.TaskHistory) error {
	return m.Called(ctx, task, entries).Error(0)
}
func (m *mockTaskStorage) ListHistory(ctx context.Context, taskID uuid.UUID) ([]domain.TaskHistory, error) {
	args := m.Called(ctx, taskID)
	return args.Get(0).([]domain.TaskHistory), args.Error(1)
}

type mockMemberChecker struct{ mock.Mock }

func (m *mockMemberChecker) GetMemberRole(ctx context.Context, teamID, userID uuid.UUID) (domain.Role, error) {
	args := m.Called(ctx, teamID, userID)
	return args.Get(0).(domain.Role), args.Error(1)
}

type mockCache struct{ mock.Mock }

func (m *mockCache) Get(ctx context.Context, key string) ([]byte, error) {
	args := m.Called(ctx, key)
	b, _ := args.Get(0).([]byte)
	return b, args.Error(1)
}
func (m *mockCache) Set(ctx context.Context, key string, data []byte, ttl time.Duration) error {
	return m.Called(ctx, key, data, ttl).Error(0)
}
func (m *mockCache) Invalidate(ctx context.Context, pattern string) error {
	return m.Called(ctx, pattern).Error(0)
}

var ctx = context.Background()

func newSvc(store *mockTaskStorage, members *mockMemberChecker, cache *mockCache) *Service {
	return New(store, store, store, store, members, cache, 5*time.Minute)
}

func TestCreate_NotMember_Forbidden(t *testing.T) {
	store := &mockTaskStorage{}
	members := &mockMemberChecker{}
	cache := &mockCache{}

	teamID, callerID := uuid.New(), uuid.New()
	members.On("GetMemberRole", mock.Anything, teamID, callerID).
		Return(domain.Role(""), domain.ErrNotFound)

	_, err := newSvc(store, members, cache).Create(ctx, callerID, domain.CreateTaskInput{
		TeamID: teamID, Title: "Fix bug",
	})

	assert.ErrorIs(t, err, domain.ErrForbidden)
	store.AssertNotCalled(t, "SaveTask")
}

func TestCreate_OK(t *testing.T) {
	store := &mockTaskStorage{}
	members := &mockMemberChecker{}
	cache := &mockCache{}

	teamID, callerID := uuid.New(), uuid.New()
	members.On("GetMemberRole", mock.Anything, teamID, callerID).
		Return(domain.RoleMember, nil)
	store.On("SaveTask", mock.Anything, mock.AnythingOfType("domain.Task")).
		Return(nil)
	cache.On("Invalidate", mock.Anything, mock.AnythingOfType("string")).
		Return(nil)

	task, err := newSvc(store, members, cache).Create(ctx, callerID, domain.CreateTaskInput{
		TeamID: teamID, Title: "Fix bug",
	})

	assert.NoError(t, err)
	assert.Equal(t, "Fix bug", task.Title)
	assert.Equal(t, domain.StatusTodo, task.Status)
	assert.Equal(t, domain.PriorityMedium, task.Priority)
}

func TestCreate_AssigneeNotMember_Forbidden(t *testing.T) {
	store := &mockTaskStorage{}
	members := &mockMemberChecker{}
	cache := &mockCache{}

	teamID, callerID, assigneeID := uuid.New(), uuid.New(), uuid.New()
	members.On("GetMemberRole", mock.Anything, teamID, callerID).Return(domain.RoleMember, nil)
	members.On("GetMemberRole", mock.Anything, teamID, assigneeID).Return(domain.Role(""), domain.ErrNotFound)

	_, err := newSvc(store, members, cache).Create(ctx, callerID, domain.CreateTaskInput{
		TeamID: teamID, Title: "Task", AssigneeID: &assigneeID,
	})

	assert.ErrorIs(t, err, domain.ErrForbidden)
	store.AssertNotCalled(t, "SaveTask")
}

func TestCreate_AssigneeMember_OK(t *testing.T) {
	store := &mockTaskStorage{}
	members := &mockMemberChecker{}
	cache := &mockCache{}

	teamID, callerID, assigneeID := uuid.New(), uuid.New(), uuid.New()
	members.On("GetMemberRole", mock.Anything, teamID, callerID).Return(domain.RoleMember, nil)
	members.On("GetMemberRole", mock.Anything, teamID, assigneeID).Return(domain.RoleMember, nil)
	store.On("SaveTask", mock.Anything, mock.AnythingOfType("domain.Task")).Return(nil)
	cache.On("Invalidate", mock.Anything, mock.AnythingOfType("string")).Return(nil)

	task, err := newSvc(store, members, cache).Create(ctx, callerID, domain.CreateTaskInput{
		TeamID: teamID, Title: "Task", AssigneeID: &assigneeID,
	})

	assert.NoError(t, err)
	assert.Equal(t, &assigneeID, task.AssigneeID)
}

func TestUpdate_TwoFieldsChanged_TwoHistoryRecords(t *testing.T) {
	store := &mockTaskStorage{}
	members := &mockMemberChecker{}
	cache := &mockCache{}

	taskID, teamID, callerID := uuid.New(), uuid.New(), uuid.New()
	existing := domain.Task{
		ID:       taskID,
		TeamID:   teamID,
		Title:    "Old Title",
		Status:   domain.StatusTodo,
		Priority: domain.PriorityMedium,
	}

	store.On("GetTaskByID", mock.Anything, taskID).Return(existing, nil)
	members.On("GetMemberRole", mock.Anything, teamID, callerID).Return(domain.RoleMember, nil)
	store.On("UpdateTaskWithHistory", mock.Anything, mock.AnythingOfType("domain.Task"),
		mock.MatchedBy(func(entries []domain.TaskHistory) bool { return len(entries) == 2 })).
		Return(nil)
	cache.On("Invalidate", mock.Anything, mock.AnythingOfType("string")).Return(nil)

	newTitle := "New Title"
	newStatus := domain.StatusInProgress
	_, err := newSvc(store, members, cache).Update(ctx, callerID, taskID, domain.UpdateTaskInput{
		Title:  &newTitle,
		Status: &newStatus,
	})

	assert.NoError(t, err)
	store.AssertCalled(t, "UpdateTaskWithHistory", mock.Anything, mock.Anything,
		mock.MatchedBy(func(entries []domain.TaskHistory) bool { return len(entries) == 2 }))
}

func TestUpdate_SameValues_NoHistoryRecords(t *testing.T) {
	store := &mockTaskStorage{}
	members := &mockMemberChecker{}
	cache := &mockCache{}

	taskID, teamID, callerID := uuid.New(), uuid.New(), uuid.New()
	existing := domain.Task{
		ID:       taskID,
		TeamID:   teamID,
		Title:    "Same Title",
		Status:   domain.StatusTodo,
		Priority: domain.PriorityMedium,
	}

	store.On("GetTaskByID", mock.Anything, taskID).Return(existing, nil)
	members.On("GetMemberRole", mock.Anything, teamID, callerID).Return(domain.RoleMember, nil)
	store.On("UpdateTaskWithHistory", mock.Anything, mock.AnythingOfType("domain.Task"),
		mock.MatchedBy(func(entries []domain.TaskHistory) bool { return len(entries) == 0 })).
		Return(nil)
	cache.On("Invalidate", mock.Anything, mock.AnythingOfType("string")).Return(nil)

	sameTitle := "Same Title"
	sameStatus := domain.StatusTodo
	_, err := newSvc(store, members, cache).Update(ctx, callerID, taskID, domain.UpdateTaskInput{
		Title:  &sameTitle,
		Status: &sameStatus,
	})

	assert.NoError(t, err)
}

func TestList_NoTeamID_Forbidden(t *testing.T) {
	store, members, cache := &mockTaskStorage{}, &mockMemberChecker{}, &mockCache{}
	callerID := uuid.New()

	_, _, err := newSvc(store, members, cache).List(ctx, callerID, domain.TaskFilter{Page: 1, Limit: 20})

	assert.ErrorIs(t, err, domain.ErrForbidden)
	store.AssertNotCalled(t, "ListTasks")
}

func TestList_NotMember_Forbidden(t *testing.T) {
	store, members, cache := &mockTaskStorage{}, &mockMemberChecker{}, &mockCache{}
	callerID, teamID := uuid.New(), uuid.New()
	members.On("GetMemberRole", mock.Anything, teamID, callerID).Return(domain.Role(""), domain.ErrNotFound)

	_, _, err := newSvc(store, members, cache).List(ctx, callerID, domain.TaskFilter{TeamID: &teamID, Page: 1, Limit: 20})

	assert.ErrorIs(t, err, domain.ErrForbidden)
	store.AssertNotCalled(t, "ListTasks")
}

func TestList_CacheHit_StorageNotCalled(t *testing.T) {
	store := &mockTaskStorage{}
	members := &mockMemberChecker{}
	cache := &mockCache{}

	callerID, teamID := uuid.New(), uuid.New()
	members.On("GetMemberRole", mock.Anything, teamID, callerID).Return(domain.RoleMember, nil)

	payload, _ := json.Marshal(struct {
		Tasks []domain.Task `json:"tasks"`
		Total int           `json:"total"`
	}{Tasks: []domain.Task{{ID: uuid.New(), Title: "cached"}}, Total: 1})

	cache.On("Get", mock.Anything, mock.AnythingOfType("string")).Return(payload, nil)

	tasks, total, err := newSvc(store, members, cache).List(ctx, callerID, domain.TaskFilter{TeamID: &teamID, Page: 1, Limit: 20})

	assert.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Len(t, tasks, 1)
	store.AssertNotCalled(t, "ListTasks")
}

func TestList_CacheMiss_StorageCalled(t *testing.T) {
	store := &mockTaskStorage{}
	members := &mockMemberChecker{}
	cache := &mockCache{}

	callerID, teamID := uuid.New(), uuid.New()
	members.On("GetMemberRole", mock.Anything, teamID, callerID).Return(domain.RoleMember, nil)
	cache.On("Get", mock.Anything, mock.AnythingOfType("string")).
		Return(nil, errors.New("miss"))
	store.On("ListTasks", mock.Anything, mock.AnythingOfType("domain.TaskFilter")).
		Return([]domain.Task{{ID: uuid.New()}}, 1, nil)
	cache.On("Set", mock.Anything, mock.AnythingOfType("string"), mock.Anything, mock.AnythingOfType("time.Duration")).
		Return(nil)

	tasks, total, err := newSvc(store, members, cache).List(ctx, callerID, domain.TaskFilter{TeamID: &teamID, Page: 1, Limit: 20})

	assert.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Len(t, tasks, 1)
	store.AssertCalled(t, "ListTasks", mock.Anything, mock.AnythingOfType("domain.TaskFilter"))
	cache.AssertCalled(t, "Set", mock.Anything, mock.AnythingOfType("string"), mock.Anything, mock.AnythingOfType("time.Duration"))
}

func TestList_StorageFails(t *testing.T) {
	store := &mockTaskStorage{}
	members := &mockMemberChecker{}
	cache := &mockCache{}

	callerID, teamID := uuid.New(), uuid.New()
	members.On("GetMemberRole", mock.Anything, teamID, callerID).Return(domain.RoleMember, nil)
	cache.On("Get", mock.Anything, mock.AnythingOfType("string")).
		Return(nil, errors.New("miss"))
	store.On("ListTasks", mock.Anything, mock.AnythingOfType("domain.TaskFilter")).
		Return([]domain.Task(nil), 0, errors.New("db error"))

	_, _, err := newSvc(store, members, cache).List(ctx, callerID, domain.TaskFilter{TeamID: &teamID, Page: 1, Limit: 20})

	assert.Error(t, err)
}

func TestCreate_SaveTaskFails(t *testing.T) {
	store := &mockTaskStorage{}
	members := &mockMemberChecker{}
	cache := &mockCache{}

	teamID, callerID := uuid.New(), uuid.New()
	members.On("GetMemberRole", mock.Anything, teamID, callerID).
		Return(domain.RoleMember, nil)
	store.On("SaveTask", mock.Anything, mock.AnythingOfType("domain.Task")).
		Return(errors.New("db error"))

	_, err := newSvc(store, members, cache).Create(ctx, callerID, domain.CreateTaskInput{
		TeamID: teamID, Title: "Fix bug",
	})

	assert.Error(t, err)
}

func TestHistory_OK(t *testing.T) {
	store := &mockTaskStorage{}
	members := &mockMemberChecker{}
	cache := &mockCache{}

	taskID, teamID, callerID := uuid.New(), uuid.New(), uuid.New()
	store.On("GetTaskByID", mock.Anything, taskID).
		Return(domain.Task{ID: taskID, TeamID: teamID}, nil)
	members.On("GetMemberRole", mock.Anything, teamID, callerID).
		Return(domain.RoleMember, nil)
	store.On("ListHistory", mock.Anything, taskID).
		Return([]domain.TaskHistory{{ID: 1, TaskID: taskID}}, nil)

	entries, err := newSvc(store, members, cache).History(ctx, callerID, taskID)

	assert.NoError(t, err)
	assert.Len(t, entries, 1)
}

func TestHistory_NotMember_Forbidden(t *testing.T) {
	store := &mockTaskStorage{}
	members := &mockMemberChecker{}
	cache := &mockCache{}

	taskID, teamID, callerID := uuid.New(), uuid.New(), uuid.New()
	store.On("GetTaskByID", mock.Anything, taskID).
		Return(domain.Task{ID: taskID, TeamID: teamID}, nil)
	members.On("GetMemberRole", mock.Anything, teamID, callerID).
		Return(domain.Role(""), domain.ErrNotFound)

	_, err := newSvc(store, members, cache).History(ctx, callerID, taskID)

	assert.ErrorIs(t, err, domain.ErrForbidden)
}

func TestHistory_TaskNotFound(t *testing.T) {
	store := &mockTaskStorage{}
	members := &mockMemberChecker{}
	cache := &mockCache{}

	taskID, callerID := uuid.New(), uuid.New()
	store.On("GetTaskByID", mock.Anything, taskID).
		Return(domain.Task{}, domain.ErrNotFound)

	_, err := newSvc(store, members, cache).History(ctx, callerID, taskID)

	assert.Error(t, err)
}

func TestUpdate_NotMember_Forbidden(t *testing.T) {
	store := &mockTaskStorage{}
	members := &mockMemberChecker{}
	cache := &mockCache{}

	taskID, teamID, callerID := uuid.New(), uuid.New(), uuid.New()
	store.On("GetTaskByID", mock.Anything, taskID).
		Return(domain.Task{ID: taskID, TeamID: teamID}, nil)
	members.On("GetMemberRole", mock.Anything, teamID, callerID).
		Return(domain.Role(""), domain.ErrNotFound)

	newTitle := "whatever"
	_, err := newSvc(store, members, cache).Update(ctx, callerID, taskID, domain.UpdateTaskInput{
		Title: &newTitle,
	})

	assert.ErrorIs(t, err, domain.ErrForbidden)
	store.AssertNotCalled(t, "UpdateTaskWithHistory")
}

func TestUpdate_AssigneeNotMember_Forbidden(t *testing.T) {
	store := &mockTaskStorage{}
	members := &mockMemberChecker{}
	cache := &mockCache{}

	taskID, teamID, callerID, assigneeID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	store.On("GetTaskByID", mock.Anything, taskID).
		Return(domain.Task{ID: taskID, TeamID: teamID, Title: "T", Status: domain.StatusTodo, Priority: domain.PriorityMedium}, nil)
	members.On("GetMemberRole", mock.Anything, teamID, callerID).Return(domain.RoleMember, nil)
	members.On("GetMemberRole", mock.Anything, teamID, assigneeID).Return(domain.Role(""), domain.ErrNotFound)

	_, err := newSvc(store, members, cache).Update(ctx, callerID, taskID, domain.UpdateTaskInput{
		AssigneeID: &assigneeID,
	})

	assert.ErrorIs(t, err, domain.ErrForbidden)
	store.AssertNotCalled(t, "UpdateTaskWithHistory")
}

func TestUpdate_AllFieldsChanged(t *testing.T) {
	store := &mockTaskStorage{}
	members := &mockMemberChecker{}
	cache := &mockCache{}

	taskID, teamID, callerID := uuid.New(), uuid.New(), uuid.New()
	oldAssignee := uuid.New()
	newAssignee := uuid.New()
	existing := domain.Task{
		ID:          taskID,
		TeamID:      teamID,
		Title:       "Old",
		Status:      domain.StatusTodo,
		Priority:    domain.PriorityLow,
		Description: strPtr("old desc"),
		AssigneeID:  &oldAssignee,
	}

	store.On("GetTaskByID", mock.Anything, taskID).Return(existing, nil)
	members.On("GetMemberRole", mock.Anything, teamID, callerID).Return(domain.RoleOwner, nil)
	members.On("GetMemberRole", mock.Anything, teamID, newAssignee).Return(domain.RoleMember, nil)
	store.On("UpdateTaskWithHistory", mock.Anything, mock.AnythingOfType("domain.Task"),
		mock.MatchedBy(func(entries []domain.TaskHistory) bool { return len(entries) == 6 })).
		Return(nil)
	cache.On("Invalidate", mock.Anything, mock.AnythingOfType("string")).Return(nil)

	newTitle := "New"
	newStatus := domain.StatusDone
	newPriority := domain.PriorityHigh
	newDesc := "new desc"
	now := time.Now()
	_, err := newSvc(store, members, cache).Update(ctx, callerID, taskID, domain.UpdateTaskInput{
		Title:       &newTitle,
		Status:      &newStatus,
		Priority:    &newPriority,
		AssigneeID:  &newAssignee,
		Description: &newDesc,
		DueDate:     &now,
	})

	assert.NoError(t, err)
	// title + status + priority + assignee + description + due_date = 6 history entries
	store.AssertCalled(t, "UpdateTaskWithHistory", mock.Anything, mock.Anything,
		mock.MatchedBy(func(entries []domain.TaskHistory) bool { return len(entries) == 6 }))
}

func strPtr(s string) *string { return &s }

func TestCreate_AssigneeCheckError(t *testing.T) {
	store, members, cache := &mockTaskStorage{}, &mockMemberChecker{}, &mockCache{}
	teamID, callerID, assigneeID := uuid.New(), uuid.New(), uuid.New()
	members.On("GetMemberRole", mock.Anything, teamID, callerID).Return(domain.RoleMember, nil)
	members.On("GetMemberRole", mock.Anything, teamID, assigneeID).Return(domain.Role(""), errors.New("db error"))

	_, err := newSvc(store, members, cache).Create(ctx, callerID, domain.CreateTaskInput{
		TeamID: teamID, Title: "Task", AssigneeID: &assigneeID,
	})

	assert.Error(t, err)
	assert.NotErrorIs(t, err, domain.ErrForbidden)
	store.AssertNotCalled(t, "SaveTask")
}

func TestList_MemberCheckError(t *testing.T) {
	store, members, cache := &mockTaskStorage{}, &mockMemberChecker{}, &mockCache{}
	callerID, teamID := uuid.New(), uuid.New()
	members.On("GetMemberRole", mock.Anything, teamID, callerID).Return(domain.Role(""), errors.New("db error"))

	_, _, err := newSvc(store, members, cache).List(ctx, callerID, domain.TaskFilter{TeamID: &teamID, Page: 1, Limit: 10})

	assert.Error(t, err)
	assert.NotErrorIs(t, err, domain.ErrForbidden)
	store.AssertNotCalled(t, "ListTasks")
}

func TestUpdate_AssigneeCheckError(t *testing.T) {
	store, members, cache := &mockTaskStorage{}, &mockMemberChecker{}, &mockCache{}
	taskID, teamID, callerID, assigneeID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	store.On("GetTaskByID", mock.Anything, taskID).
		Return(domain.Task{ID: taskID, TeamID: teamID, Title: "T", Status: domain.StatusTodo, Priority: domain.PriorityMedium}, nil)
	members.On("GetMemberRole", mock.Anything, teamID, callerID).Return(domain.RoleMember, nil)
	members.On("GetMemberRole", mock.Anything, teamID, assigneeID).Return(domain.Role(""), errors.New("db error"))

	_, err := newSvc(store, members, cache).Update(ctx, callerID, taskID, domain.UpdateTaskInput{
		AssigneeID: &assigneeID,
	})

	assert.Error(t, err)
	assert.NotErrorIs(t, err, domain.ErrForbidden)
	store.AssertNotCalled(t, "UpdateTaskWithHistory")
}

func TestUpdate_DescriptionSetFromNil(t *testing.T) {
	store, members, cache := &mockTaskStorage{}, &mockMemberChecker{}, &mockCache{}
	taskID, teamID, callerID := uuid.New(), uuid.New(), uuid.New()
	store.On("GetTaskByID", mock.Anything, taskID).
		Return(domain.Task{ID: taskID, TeamID: teamID, Title: "T", Status: domain.StatusTodo, Priority: domain.PriorityMedium, Description: nil}, nil)
	members.On("GetMemberRole", mock.Anything, teamID, callerID).Return(domain.RoleMember, nil)
	store.On("UpdateTaskWithHistory", mock.Anything, mock.AnythingOfType("domain.Task"),
		mock.MatchedBy(func(entries []domain.TaskHistory) bool {
			return len(entries) == 1 && entries[0].Field == "description"
		})).Return(nil)
	cache.On("Invalidate", mock.Anything, mock.AnythingOfType("string")).Return(nil)

	desc := "brand new description"
	_, err := newSvc(store, members, cache).Update(ctx, callerID, taskID, domain.UpdateTaskInput{
		Description: &desc,
	})

	assert.NoError(t, err)
}

func TestUpdate_DueDateChangedBothNonNil(t *testing.T) {
	store, members, cache := &mockTaskStorage{}, &mockMemberChecker{}, &mockCache{}
	taskID, teamID, callerID := uuid.New(), uuid.New(), uuid.New()
	oldDue := time.Now().Add(-24 * time.Hour).UTC()
	newDue := time.Now().UTC()
	store.On("GetTaskByID", mock.Anything, taskID).
		Return(domain.Task{ID: taskID, TeamID: teamID, Title: "T", Status: domain.StatusTodo, Priority: domain.PriorityMedium, DueDate: &oldDue}, nil)
	members.On("GetMemberRole", mock.Anything, teamID, callerID).Return(domain.RoleMember, nil)
	store.On("UpdateTaskWithHistory", mock.Anything, mock.AnythingOfType("domain.Task"),
		mock.MatchedBy(func(entries []domain.TaskHistory) bool {
			return len(entries) == 1 && entries[0].Field == "due_date"
		})).Return(nil)
	cache.On("Invalidate", mock.Anything, mock.AnythingOfType("string")).Return(nil)

	_, err := newSvc(store, members, cache).Update(ctx, callerID, taskID, domain.UpdateTaskInput{
		DueDate: &newDue,
	})

	assert.NoError(t, err)
}

func TestCreate_CustomPriority(t *testing.T) {
	store, members, cache := &mockTaskStorage{}, &mockMemberChecker{}, &mockCache{}
	teamID, callerID := uuid.New(), uuid.New()
	members.On("GetMemberRole", mock.Anything, teamID, callerID).Return(domain.RoleMember, nil)
	store.On("SaveTask", mock.Anything, mock.AnythingOfType("domain.Task")).Return(nil)
	cache.On("Invalidate", mock.Anything, mock.AnythingOfType("string")).Return(nil)

	task, err := newSvc(store, members, cache).Create(ctx, callerID, domain.CreateTaskInput{
		TeamID: teamID, Title: "Urgent", Priority: domain.PriorityHigh,
	})

	assert.NoError(t, err)
	assert.Equal(t, domain.PriorityHigh, task.Priority)
}

func TestCreate_MembershipCheckFails(t *testing.T) {
	store, members, cache := &mockTaskStorage{}, &mockMemberChecker{}, &mockCache{}
	teamID, callerID := uuid.New(), uuid.New()
	members.On("GetMemberRole", mock.Anything, teamID, callerID).
		Return(domain.Role(""), errors.New("db error"))

	_, err := newSvc(store, members, cache).Create(ctx, callerID, domain.CreateTaskInput{TeamID: teamID, Title: "x"})

	assert.Error(t, err)
	store.AssertNotCalled(t, "SaveTask")
}

func TestCreate_CacheInvalidateFails(t *testing.T) {
	store, members, cache := &mockTaskStorage{}, &mockMemberChecker{}, &mockCache{}
	teamID, callerID := uuid.New(), uuid.New()
	members.On("GetMemberRole", mock.Anything, teamID, callerID).Return(domain.RoleMember, nil)
	store.On("SaveTask", mock.Anything, mock.AnythingOfType("domain.Task")).Return(nil)
	cache.On("Invalidate", mock.Anything, mock.AnythingOfType("string")).Return(errors.New("redis down"))

	task, err := newSvc(store, members, cache).Create(ctx, callerID, domain.CreateTaskInput{TeamID: teamID, Title: "x"})

	assert.NoError(t, err, "cache failure must not fail the operation")
	assert.Equal(t, "x", task.Title)
}

func TestList_LimitClamped(t *testing.T) {
	store, members, cache := &mockTaskStorage{}, &mockMemberChecker{}, &mockCache{}
	callerID, teamID := uuid.New(), uuid.New()
	members.On("GetMemberRole", mock.Anything, teamID, callerID).Return(domain.RoleMember, nil)
	cache.On("Get", mock.Anything, mock.AnythingOfType("string")).Return(nil, errors.New("miss"))
	store.On("ListTasks", mock.Anything, mock.AnythingOfType("domain.TaskFilter")).Return([]domain.Task{}, 0, nil)
	cache.On("Set", mock.Anything, mock.AnythingOfType("string"), mock.Anything, mock.AnythingOfType("time.Duration")).Return(nil)

	_, _, err := newSvc(store, members, cache).List(ctx, callerID, domain.TaskFilter{TeamID: &teamID, Page: 1, Limit: 200})

	assert.NoError(t, err)
	store.AssertCalled(t, "ListTasks", mock.Anything, mock.MatchedBy(func(f domain.TaskFilter) bool {
		return f.Limit == 100
	}))
}

func TestList_CacheSetFails(t *testing.T) {
	store, members, cache := &mockTaskStorage{}, &mockMemberChecker{}, &mockCache{}
	callerID, teamID := uuid.New(), uuid.New()
	members.On("GetMemberRole", mock.Anything, teamID, callerID).Return(domain.RoleMember, nil)
	cache.On("Get", mock.Anything, mock.AnythingOfType("string")).Return(nil, errors.New("miss"))
	store.On("ListTasks", mock.Anything, mock.AnythingOfType("domain.TaskFilter")).Return([]domain.Task{{ID: uuid.New()}}, 1, nil)
	cache.On("Set", mock.Anything, mock.AnythingOfType("string"), mock.Anything, mock.AnythingOfType("time.Duration")).Return(errors.New("redis down"))

	tasks, total, err := newSvc(store, members, cache).List(ctx, callerID, domain.TaskFilter{TeamID: &teamID, Page: 1, Limit: 10})

	assert.NoError(t, err, "cache failure must not fail the operation")
	assert.Equal(t, 1, total)
	assert.Len(t, tasks, 1)
}

func TestUpdate_MembershipCheckFails(t *testing.T) {
	store, members, cache := &mockTaskStorage{}, &mockMemberChecker{}, &mockCache{}
	taskID, teamID, callerID := uuid.New(), uuid.New(), uuid.New()
	store.On("GetTaskByID", mock.Anything, taskID).Return(domain.Task{ID: taskID, TeamID: teamID}, nil)
	members.On("GetMemberRole", mock.Anything, teamID, callerID).Return(domain.Role(""), errors.New("db error"))

	newTitle := "x"
	_, err := newSvc(store, members, cache).Update(ctx, callerID, taskID, domain.UpdateTaskInput{Title: &newTitle})

	assert.Error(t, err)
	store.AssertNotCalled(t, "UpdateTaskWithHistory")
}

func TestUpdate_StorageFails(t *testing.T) {
	store, members, cache := &mockTaskStorage{}, &mockMemberChecker{}, &mockCache{}
	taskID, teamID, callerID := uuid.New(), uuid.New(), uuid.New()
	store.On("GetTaskByID", mock.Anything, taskID).Return(domain.Task{ID: taskID, TeamID: teamID, Title: "old"}, nil)
	members.On("GetMemberRole", mock.Anything, teamID, callerID).Return(domain.RoleMember, nil)
	store.On("UpdateTaskWithHistory", mock.Anything, mock.AnythingOfType("domain.Task"), mock.Anything).Return(errors.New("db error"))

	newTitle := "x"
	_, err := newSvc(store, members, cache).Update(ctx, callerID, taskID, domain.UpdateTaskInput{Title: &newTitle})

	assert.Error(t, err)
}

func TestUpdate_CacheInvalidateFails(t *testing.T) {
	store, members, cache := &mockTaskStorage{}, &mockMemberChecker{}, &mockCache{}
	taskID, teamID, callerID := uuid.New(), uuid.New(), uuid.New()
	store.On("GetTaskByID", mock.Anything, taskID).Return(domain.Task{ID: taskID, TeamID: teamID, Title: "old"}, nil)
	members.On("GetMemberRole", mock.Anything, teamID, callerID).Return(domain.RoleMember, nil)
	store.On("UpdateTaskWithHistory", mock.Anything, mock.AnythingOfType("domain.Task"), mock.Anything).Return(nil)
	cache.On("Invalidate", mock.Anything, mock.AnythingOfType("string")).Return(errors.New("redis down"))

	newTitle := "new"
	task, err := newSvc(store, members, cache).Update(ctx, callerID, taskID, domain.UpdateTaskInput{Title: &newTitle})

	assert.NoError(t, err, "cache failure must not fail the operation")
	assert.Equal(t, "new", task.Title)
}

func TestHistory_MembershipCheckFails(t *testing.T) {
	store, members, cache := &mockTaskStorage{}, &mockMemberChecker{}, &mockCache{}
	taskID, teamID, callerID := uuid.New(), uuid.New(), uuid.New()
	store.On("GetTaskByID", mock.Anything, taskID).Return(domain.Task{ID: taskID, TeamID: teamID}, nil)
	members.On("GetMemberRole", mock.Anything, teamID, callerID).Return(domain.Role(""), errors.New("db error"))

	_, err := newSvc(store, members, cache).History(ctx, callerID, taskID)

	assert.Error(t, err)
}

func TestHistory_StorageFails(t *testing.T) {
	store, members, cache := &mockTaskStorage{}, &mockMemberChecker{}, &mockCache{}
	taskID, teamID, callerID := uuid.New(), uuid.New(), uuid.New()
	store.On("GetTaskByID", mock.Anything, taskID).Return(domain.Task{ID: taskID, TeamID: teamID}, nil)
	members.On("GetMemberRole", mock.Anything, teamID, callerID).Return(domain.RoleMember, nil)
	store.On("ListHistory", mock.Anything, taskID).Return([]domain.TaskHistory(nil), errors.New("db error"))

	_, err := newSvc(store, members, cache).History(ctx, callerID, taskID)

	assert.Error(t, err)
}

func TestUpdate_TaskNotFound(t *testing.T) {
	store := &mockTaskStorage{}
	members := &mockMemberChecker{}
	cache := &mockCache{}

	taskID, callerID := uuid.New(), uuid.New()
	store.On("GetTaskByID", mock.Anything, taskID).
		Return(domain.Task{}, domain.ErrNotFound)

	newTitle := "x"
	_, err := newSvc(store, members, cache).Update(ctx, callerID, taskID, domain.UpdateTaskInput{Title: &newTitle})

	assert.Error(t, err)
}

func TestUpdate_WithHistoryFails(t *testing.T) {
	store, members, cache := &mockTaskStorage{}, &mockMemberChecker{}, &mockCache{}
	taskID, teamID, callerID := uuid.New(), uuid.New(), uuid.New()
	store.On("GetTaskByID", mock.Anything, taskID).Return(domain.Task{ID: taskID, TeamID: teamID, Title: "old"}, nil)
	members.On("GetMemberRole", mock.Anything, teamID, callerID).Return(domain.RoleMember, nil)
	store.On("UpdateTaskWithHistory", mock.Anything, mock.AnythingOfType("domain.Task"), mock.Anything).Return(errors.New("db error"))

	newTitle := "new"
	_, err := newSvc(store, members, cache).Update(ctx, callerID, taskID, domain.UpdateTaskInput{Title: &newTitle})

	assert.Error(t, err, "UpdateTaskWithHistory failure must be fatal to preserve audit trail integrity")
}

func TestList_PageLimitDefaults(t *testing.T) {
	store := &mockTaskStorage{}
	members := &mockMemberChecker{}
	cache := &mockCache{}

	callerID, teamID := uuid.New(), uuid.New()
	members.On("GetMemberRole", mock.Anything, teamID, callerID).Return(domain.RoleMember, nil)
	cache.On("Get", mock.Anything, mock.AnythingOfType("string")).
		Return(nil, errors.New("miss"))
	store.On("ListTasks", mock.Anything, mock.AnythingOfType("domain.TaskFilter")).
		Return([]domain.Task{}, 0, nil)
	cache.On("Set", mock.Anything, mock.AnythingOfType("string"), mock.Anything, mock.AnythingOfType("time.Duration")).
		Return(nil)

	// page=0 and limit=0 should be normalized to 1 and 20
	_, _, err := newSvc(store, members, cache).List(ctx, callerID, domain.TaskFilter{TeamID: &teamID, Page: 0, Limit: 0})

	assert.NoError(t, err)
	store.AssertCalled(t, "ListTasks", mock.Anything, mock.MatchedBy(func(f domain.TaskFilter) bool {
		return f.Page == 1 && f.Limit == 20
	}))
}

// TestList_AllFilterFields exercises the TeamID, Status, and AssigneeID branches
// inside cacheKey, bringing its coverage from 70% to 100%.
func TestList_AllFilterFields(t *testing.T) {
	store, members, cache := &mockTaskStorage{}, &mockMemberChecker{}, &mockCache{}
	callerID := uuid.New()
	teamID := uuid.New()
	status := domain.StatusTodo
	assigneeID := uuid.New()

	members.On("GetMemberRole", mock.Anything, teamID, callerID).Return(domain.RoleMember, nil)
	cache.On("Get", mock.Anything, mock.AnythingOfType("string")).
		Return(nil, errors.New("miss"))
	store.On("ListTasks", mock.Anything, mock.AnythingOfType("domain.TaskFilter")).
		Return([]domain.Task{}, 0, nil)
	cache.On("Set", mock.Anything, mock.AnythingOfType("string"), mock.Anything, mock.AnythingOfType("time.Duration")).
		Return(nil)

	_, _, err := newSvc(store, members, cache).List(ctx, callerID, domain.TaskFilter{
		TeamID:     &teamID,
		Status:     &status,
		AssigneeID: &assigneeID,
		Page:       1,
		Limit:      10,
	})
	assert.NoError(t, err)
}
