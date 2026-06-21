//go:build integration

package tests

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lkmavi/team-tasks/tests/suite"
)

func TestTask_Create_OK(t *testing.T) {
	_, s := suite.New(t)

	email, pass := uniqueEmail(), "pass123"
	register(t, s, email, uniqueName(), pass)
	token := login(t, s, email, pass)
	teamID := createTeam(t, s, "TaskTeam", token)

	resp := s.POST(t, "/api/v1/tasks", map[string]any{
		"team_id":  teamID,
		"title":    "Implement feature X",
		"priority": "high",
	}, token)
	defer resp.Body.Close()

	var body struct {
		ID       string `json:"id"`
		Title    string `json:"title"`
		Status   string `json:"status"`
		Priority string `json:"priority"`
	}
	suite.Decode(t, resp, &body)

	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	assert.NotEmpty(t, body.ID)
	assert.Equal(t, "Implement feature X", body.Title)
	assert.Equal(t, "todo", body.Status, "new task must default to todo")
	assert.Equal(t, "high", body.Priority)
}

func TestTask_Create_Unauthorized(t *testing.T) {
	_, s := suite.New(t)

	resp := s.POST(t, "/api/v1/tasks", map[string]any{
		"team_id": uuid.New().String(), "title": "fail",
	}, "")
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

// TestTask_Create_NonMember_Forbidden verifies that a user not belonging to the team
// cannot create tasks in it.
func TestTask_Create_NonMember_Forbidden(t *testing.T) {
	_, s := suite.New(t)

	ownerEmail, ownerPass := uniqueEmail(), "pass123"
	register(t, s, ownerEmail, uniqueName(), ownerPass)
	ownerToken := login(t, s, ownerEmail, ownerPass)
	teamID := createTeam(t, s, "OwnerTeam", ownerToken)

	outsiderEmail, outsiderPass := uniqueEmail(), "pass456"
	register(t, s, outsiderEmail, uniqueName(), outsiderPass)
	outsiderToken := login(t, s, outsiderEmail, outsiderPass)

	resp := s.POST(t, "/api/v1/tasks", map[string]any{
		"team_id": teamID, "title": "stealth task",
	}, outsiderToken)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestTask_List_WithTeamFilter(t *testing.T) {
	_, s := suite.New(t)

	email, pass := uniqueEmail(), "pass123"
	register(t, s, email, uniqueName(), pass)
	token := login(t, s, email, pass)
	teamID := createTeam(t, s, "FilterTeam", token)

	createTask(t, s, teamID, "Task A", token)
	createTask(t, s, teamID, "Task B", token)

	resp := s.GET(t, "/api/v1/tasks?team_id="+teamID+"&page=1&limit=10", token)
	defer resp.Body.Close()

	var body struct {
		Tasks []map[string]any `json:"tasks"`
		Total int              `json:"total"`
	}
	suite.Decode(t, resp, &body)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, 2, body.Total)
	assert.Len(t, body.Tasks, 2)
}

func TestTask_List_StatusFilter(t *testing.T) {
	_, s := suite.New(t)

	email, pass := uniqueEmail(), "pass123"
	register(t, s, email, uniqueName(), pass)
	token := login(t, s, email, pass)
	teamID := createTeam(t, s, "StatusTeam", token)

	taskID := createTask(t, s, teamID, "In progress task", token)

	// advance the task to in_progress
	upd := s.PUT(t, "/api/v1/tasks/"+taskID, map[string]any{"status": "in_progress"}, token)
	upd.Body.Close()

	// filter todo → should return 0
	todoResp := s.GET(t, "/api/v1/tasks?team_id="+teamID+"&status=todo", token)
	defer todoResp.Body.Close()
	var todoBody struct {
		Total int `json:"total"`
	}
	suite.Decode(t, todoResp, &todoBody)
	assert.Equal(t, 0, todoBody.Total)

	// filter in_progress → should return 1
	ipResp := s.GET(t, "/api/v1/tasks?team_id="+teamID+"&status=in_progress", token)
	defer ipResp.Body.Close()
	var ipBody struct {
		Total int `json:"total"`
	}
	suite.Decode(t, ipResp, &ipBody)
	assert.Equal(t, 1, ipBody.Total)
}

func TestTask_List_Unauthorized(t *testing.T) {
	_, s := suite.New(t)

	resp := s.GET(t, "/api/v1/tasks", "")
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestTask_Update_OK(t *testing.T) {
	_, s := suite.New(t)

	email, pass := uniqueEmail(), "pass123"
	register(t, s, email, uniqueName(), pass)
	token := login(t, s, email, pass)
	teamID := createTeam(t, s, "UpdateTeam", token)
	taskID := createTask(t, s, teamID, "Original title", token)

	newTitle := "Updated title"
	resp := s.PUT(t, "/api/v1/tasks/"+taskID, map[string]any{
		"title": newTitle, "status": "in_progress",
	}, token)
	defer resp.Body.Close()

	var body struct {
		Title  string `json:"title"`
		Status string `json:"status"`
	}
	suite.Decode(t, resp, &body)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, newTitle, body.Title)
	assert.Equal(t, "in_progress", body.Status)
}

func TestTask_Update_Unauthorized(t *testing.T) {
	_, s := suite.New(t)

	resp := s.PUT(t, "/api/v1/tasks/"+uuid.New().String(),
		map[string]any{"title": "x"}, "")
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestTask_Update_NotFound(t *testing.T) {
	_, s := suite.New(t)

	email, pass := uniqueEmail(), "pass123"
	register(t, s, email, uniqueName(), pass)
	token := login(t, s, email, pass)

	resp := s.PUT(t, "/api/v1/tasks/"+uuid.New().String(),
		map[string]any{"title": "ghost"}, token)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// TestTask_Update_NonMember_Forbidden verifies that a user outside the team
// cannot update tasks in it.
func TestTask_Update_NonMember_Forbidden(t *testing.T) {
	_, s := suite.New(t)

	ownerEmail, ownerPass := uniqueEmail(), "pass123"
	register(t, s, ownerEmail, uniqueName(), ownerPass)
	ownerToken := login(t, s, ownerEmail, ownerPass)
	teamID := createTeam(t, s, "TeamY", ownerToken)
	taskID := createTask(t, s, teamID, "Owned task", ownerToken)

	outsiderEmail, outsiderPass := uniqueEmail(), "pass456"
	register(t, s, outsiderEmail, uniqueName(), outsiderPass)
	outsiderToken := login(t, s, outsiderEmail, outsiderPass)

	resp := s.PUT(t, "/api/v1/tasks/"+taskID,
		map[string]any{"title": "hijacked"}, outsiderToken)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestTask_History_OK(t *testing.T) {
	_, s := suite.New(t)

	email, pass := uniqueEmail(), "pass123"
	register(t, s, email, uniqueName(), pass)
	token := login(t, s, email, pass)
	teamID := createTeam(t, s, "HistTeam", token)
	taskID := createTask(t, s, teamID, "Task to track", token)

	// two distinct field changes → two history records
	upd := s.PUT(t, "/api/v1/tasks/"+taskID, map[string]any{
		"title": "Renamed task", "status": "in_progress",
	}, token)
	upd.Body.Close()

	resp := s.GET(t, "/api/v1/tasks/"+taskID+"/history", token)
	defer resp.Body.Close()

	var body struct {
		History []map[string]any `json:"history"`
	}
	suite.Decode(t, resp, &body)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	require.Len(t, body.History, 2, "title and status each produce one history entry")

	fields := []string{body.History[0]["field"].(string), body.History[1]["field"].(string)}
	assert.ElementsMatch(t, []string{"title", "status"}, fields)
}

func TestTask_History_Unauthorized(t *testing.T) {
	_, s := suite.New(t)

	resp := s.GET(t, "/api/v1/tasks/"+uuid.New().String()+"/history", "")
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

// TestTask_History_NonMember_Forbidden verifies that a user not in the team
// cannot read the task change history.
func TestTask_History_NonMember_Forbidden(t *testing.T) {
	_, s := suite.New(t)

	ownerEmail, ownerPass := uniqueEmail(), "pass123"
	register(t, s, ownerEmail, uniqueName(), ownerPass)
	ownerToken := login(t, s, ownerEmail, ownerPass)
	teamID := createTeam(t, s, "HistForbidTeam", ownerToken)
	taskID := createTask(t, s, teamID, "Restricted task", ownerToken)

	outsiderEmail, outsiderPass := uniqueEmail(), "pass456"
	register(t, s, outsiderEmail, uniqueName(), outsiderPass)
	outsiderToken := login(t, s, outsiderEmail, outsiderPass)

	resp := s.GET(t, "/api/v1/tasks/"+taskID+"/history", outsiderToken)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestTask_Create_WithAssigneeAndDueDate(t *testing.T) {
	_, s := suite.New(t)

	email, pass := uniqueEmail(), "pass123"
	register(t, s, email, uniqueName(), pass)
	token := login(t, s, email, pass)
	teamID := createTeam(t, s, "OptFieldsTeam", token)
	userID := userIDFromDB(t, s, email)

	resp := s.POST(t, "/api/v1/tasks", map[string]any{
		"team_id":     teamID,
		"title":       "Task with all fields",
		"assignee_id": userID,
		"due_date":    "2026-12-31",
	}, token)
	defer resp.Body.Close()

	var body struct {
		AssigneeID string `json:"assignee_id"`
		DueDate    string `json:"due_date"`
	}
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	suite.Decode(t, resp, &body)
	assert.Equal(t, userID, body.AssigneeID)
	assert.NotEmpty(t, body.DueDate)
}

func TestTask_Update_WithAllOptionalFields(t *testing.T) {
	_, s := suite.New(t)

	email, pass := uniqueEmail(), "pass123"
	register(t, s, email, uniqueName(), pass)
	token := login(t, s, email, pass)
	teamID := createTeam(t, s, "AllFieldsTeam", token)
	taskID := createTask(t, s, teamID, "Original", token)
	userID := userIDFromDB(t, s, email)

	resp := s.PUT(t, "/api/v1/tasks/"+taskID, map[string]any{
		"title":       "Updated title",
		"description": "New description",
		"priority":    "high",
		"assignee_id": userID,
		"due_date":    "2026-12-31",
	}, token)
	defer resp.Body.Close()

	var body struct {
		Title    string `json:"title"`
		Priority string `json:"priority"`
	}
	require.Equal(t, http.StatusOK, resp.StatusCode)
	suite.Decode(t, resp, &body)
	assert.Equal(t, "Updated title", body.Title)
	assert.Equal(t, "high", body.Priority)
}

func TestTask_List_WithAssigneeFilter(t *testing.T) {
	_, s := suite.New(t)

	email, pass := uniqueEmail(), "pass123"
	register(t, s, email, uniqueName(), pass)
	token := login(t, s, email, pass)
	teamID := createTeam(t, s, "AssigneeFilterTeam", token)
	userID := userIDFromDB(t, s, email)

	// unassigned task
	createTask(t, s, teamID, "Unassigned", token)

	// assigned task
	assignedResp := s.POST(t, "/api/v1/tasks", map[string]any{
		"team_id": teamID, "title": "Assigned", "assignee_id": userID,
	}, token)
	assignedResp.Body.Close()

	resp := s.GET(t, "/api/v1/tasks?team_id="+teamID+"&assignee_id="+userID, token)
	defer resp.Body.Close()

	var body struct {
		Total int `json:"total"`
	}
	require.Equal(t, http.StatusOK, resp.StatusCode)
	suite.Decode(t, resp, &body)
	assert.Equal(t, 1, body.Total)
}

func TestTask_List_CacheHit(t *testing.T) {
	_, s := suite.New(t)

	email, pass := uniqueEmail(), "pass123"
	register(t, s, email, uniqueName(), pass)
	token := login(t, s, email, pass)
	teamID := createTeam(t, s, "CacheTeam", token)
	createTask(t, s, teamID, "Cached task", token)

	const path = "/api/v1/tasks?page=1&limit=10&team_id="

	// first request: cache miss → storage called, result cached
	first := s.GET(t, path+teamID, token)
	first.Body.Close()
	require.Equal(t, http.StatusOK, first.StatusCode)

	// second request: same key → cache hit
	second := s.GET(t, path+teamID, token)
	defer second.Body.Close()

	var body struct {
		Total int `json:"total"`
	}
	require.Equal(t, http.StatusOK, second.StatusCode)
	suite.Decode(t, second, &body)
	assert.Equal(t, 1, body.Total)
}
