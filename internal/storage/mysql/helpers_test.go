package mysql

import (
	"errors"
	"testing"
	"time"

	mysqldrv "github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lkmavi/team-tasks/internal/domain"
)

// helper functions

func TestUUIDToBytes_RoundTrip(t *testing.T) {
	id := uuid.New()
	b := uuidToBytes(id)
	assert.Len(t, b, 16)
	got, err := bytesToUUID(b)
	require.NoError(t, err)
	assert.Equal(t, id, got)
}

func TestBytesToUUID_InvalidLength(t *testing.T) {
	_, err := bytesToUUID([]byte{1, 2, 3})
	assert.Error(t, err)
}

func TestBytesToUUID_EmptyBytes(t *testing.T) {
	_, err := bytesToUUID([]byte{})
	assert.Error(t, err)
}

func TestBytesToUUIDPtr_NilInput(t *testing.T) {
	ptr, err := bytesToUUIDPtr(nil)
	require.NoError(t, err)
	assert.Nil(t, ptr)
}

func TestBytesToUUIDPtr_EmptyInput(t *testing.T) {
	ptr, err := bytesToUUIDPtr([]byte{})
	require.NoError(t, err)
	assert.Nil(t, ptr)
}

func TestBytesToUUIDPtr_InvalidInput(t *testing.T) {
	_, err := bytesToUUIDPtr([]byte{1, 2, 3})
	assert.Error(t, err)
}

func TestBytesToUUIDPtr_ValidInput(t *testing.T) {
	id := uuid.New()
	ptr, err := bytesToUUIDPtr(uuidToBytes(id))
	require.NoError(t, err)
	require.NotNil(t, ptr)
	assert.Equal(t, id, *ptr)
}

func TestIsDuplicateEntry_True(t *testing.T) {
	err := &mysqldrv.MySQLError{Number: 1062, Message: "Duplicate entry"}
	assert.True(t, isDuplicateEntry(err))
}

func TestIsDuplicateEntry_False_WrongCode(t *testing.T) {
	err := &mysqldrv.MySQLError{Number: 1064, Message: "Syntax error"}
	assert.False(t, isDuplicateEntry(err))
}

func TestIsDuplicateEntry_False_NotMySQLError(t *testing.T) {
	assert.False(t, isDuplicateEntry(errors.New("something else")))
}

// converter functions

func validUUID() []byte { return uuidToBytes(uuid.New()) }

func TestToTask_InvalidID(t *testing.T) {
	_, err := toTask(taskRow{ID: []byte{1}})
	assert.Error(t, err)
}

func TestToTask_InvalidTeamID(t *testing.T) {
	_, err := toTask(taskRow{ID: validUUID(), TeamID: []byte{1}})
	assert.Error(t, err)
}

func TestToTask_InvalidCreatedBy(t *testing.T) {
	_, err := toTask(taskRow{ID: validUUID(), TeamID: validUUID(), CreatedBy: []byte{1}})
	assert.Error(t, err)
}

func TestToTask_InvalidAssigneeID(t *testing.T) {
	_, err := toTask(taskRow{
		ID: validUUID(), TeamID: validUUID(), CreatedBy: validUUID(),
		AssigneeID: []byte{1},
	})
	assert.Error(t, err)
}

func TestToTask_OK(t *testing.T) {
	id := uuid.New()
	task, err := toTask(taskRow{
		ID:        uuidToBytes(id),
		TeamID:    validUUID(),
		CreatedBy: validUUID(),
		Title:     "hello",
		Status:    domain.StatusTodo,
		Priority:  domain.PriorityMedium,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	})
	require.NoError(t, err)
	assert.Equal(t, id, task.ID)
	assert.Equal(t, "hello", task.Title)
}

func TestToTeam_InvalidID(t *testing.T) {
	_, err := toTeam(teamRow{ID: []byte{1}})
	assert.Error(t, err)
}

func TestToTeam_InvalidCreatedBy(t *testing.T) {
	_, err := toTeam(teamRow{ID: validUUID(), CreatedBy: []byte{1}})
	assert.Error(t, err)
}

func TestToTeam_OK(t *testing.T) {
	id := uuid.New()
	team, err := toTeam(teamRow{ID: uuidToBytes(id), Name: "Avengers", CreatedBy: validUUID()})
	require.NoError(t, err)
	assert.Equal(t, id, team.ID)
	assert.Equal(t, "Avengers", team.Name)
}

func TestToUser_InvalidID(t *testing.T) {
	_, err := toUser(userRow{ID: []byte{1}})
	assert.Error(t, err)
}

func TestToUser_OK(t *testing.T) {
	id := uuid.New()
	user, err := toUser(userRow{ID: uuidToBytes(id), Email: "a@b.com"})
	require.NoError(t, err)
	assert.Equal(t, id, user.ID)
	assert.Equal(t, "a@b.com", user.Email)
}

func TestToComment_InvalidTaskID(t *testing.T) {
	_, err := toComment(commentRow{TaskID: []byte{1}})
	assert.Error(t, err)
}

func TestToComment_InvalidUserID(t *testing.T) {
	_, err := toComment(commentRow{TaskID: validUUID(), UserID: []byte{1}})
	assert.Error(t, err)
}

func TestToComment_OK(t *testing.T) {
	comment, err := toComment(commentRow{
		TaskID: validUUID(), UserID: validUUID(), Body: "hello",
	})
	require.NoError(t, err)
	assert.Equal(t, "hello", comment.Body)
}

func TestToHistory_InvalidTaskID(t *testing.T) {
	_, err := toHistory(historyRow{TaskID: []byte{1}})
	assert.Error(t, err)
}

func TestToHistory_InvalidChangedBy(t *testing.T) {
	_, err := toHistory(historyRow{TaskID: validUUID(), ChangedBy: []byte{1}})
	assert.Error(t, err)
}

func TestToHistory_OK(t *testing.T) {
	h, err := toHistory(historyRow{
		TaskID:    validUUID(),
		ChangedBy: validUUID(),
		Field:     "title",
	})
	require.NoError(t, err)
	assert.Equal(t, "title", h.Field)
}

func TestBuildTaskFilter_AllNil(t *testing.T) {
	conds, args := buildTaskFilter(domain.TaskFilter{})
	assert.Empty(t, conds)
	assert.Empty(t, args)
}

func TestBuildTaskFilter_AllSet(t *testing.T) {
	teamID := uuid.New()
	status := domain.StatusTodo
	assigneeID := uuid.New()
	filter := domain.TaskFilter{
		TeamID:     &teamID,
		Status:     &status,
		AssigneeID: &assigneeID,
	}
	conds, args := buildTaskFilter(filter)
	assert.Len(t, conds, 3)
	assert.Len(t, args, 3)
}
