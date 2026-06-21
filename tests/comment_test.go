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

func TestComment_Add_OK(t *testing.T) {
	_, s := suite.New(t)

	email, pass := uniqueEmail(), "pass123"
	register(t, s, email, uniqueName(), pass)
	token := login(t, s, email, pass)
	teamID := createTeam(t, s, "CommentTeam", token)
	taskID := createTask(t, s, teamID, "Task with comment", token)

	resp := s.POST(t, "/api/v1/tasks/"+taskID+"/comments",
		map[string]any{"body": "LGTM"}, token)
	defer resp.Body.Close()

	var body struct {
		ID   int64  `json:"id"`
		Body string `json:"body"`
	}
	suite.Decode(t, resp, &body)

	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	assert.Positive(t, body.ID)
	assert.Equal(t, "LGTM", body.Body)
}

func TestComment_List_OK(t *testing.T) {
	_, s := suite.New(t)

	email, pass := uniqueEmail(), "pass123"
	register(t, s, email, uniqueName(), pass)
	token := login(t, s, email, pass)
	teamID := createTeam(t, s, "CommentListTeam", token)
	taskID := createTask(t, s, teamID, "Commented task", token)

	// add two comments
	r1 := s.POST(t, "/api/v1/tasks/"+taskID+"/comments",
		map[string]any{"body": "First"}, token)
	r1.Body.Close()
	r2 := s.POST(t, "/api/v1/tasks/"+taskID+"/comments",
		map[string]any{"body": "Second"}, token)
	r2.Body.Close()

	resp := s.GET(t, "/api/v1/tasks/"+taskID+"/comments", token)
	defer resp.Body.Close()

	var comments []map[string]any
	suite.Decode(t, resp, &comments)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	require.Len(t, comments, 2)
	assert.Equal(t, "First", comments[0]["body"])
	assert.Equal(t, "Second", comments[1]["body"])
}

func TestComment_Add_Unauthorized(t *testing.T) {
	_, s := suite.New(t)

	resp := s.POST(t, "/api/v1/tasks/"+uuid.New().String()+"/comments",
		map[string]any{"body": "nope"}, "")
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestComment_List_Unauthorized(t *testing.T) {
	_, s := suite.New(t)

	resp := s.GET(t, "/api/v1/tasks/"+uuid.New().String()+"/comments", "")
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

// TestComment_Add_TaskNotFound verifies 404 when the task doesn't exist.
func TestComment_Add_TaskNotFound(t *testing.T) {
	_, s := suite.New(t)

	email, pass := uniqueEmail(), "pass123"
	register(t, s, email, uniqueName(), pass)
	token := login(t, s, email, pass)

	resp := s.POST(t, "/api/v1/tasks/"+uuid.New().String()+"/comments",
		map[string]any{"body": "ghost comment"}, token)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// TestComment_NonMember_Forbidden verifies that a user outside the team
// cannot add or list comments on tasks within it.
func TestComment_NonMember_Forbidden(t *testing.T) {
	_, s := suite.New(t)

	ownerEmail, ownerPass := uniqueEmail(), "pass123"
	register(t, s, ownerEmail, uniqueName(), ownerPass)
	ownerToken := login(t, s, ownerEmail, ownerPass)
	teamID := createTeam(t, s, "CommentForbidTeam", ownerToken)
	taskID := createTask(t, s, teamID, "Private task", ownerToken)

	outsiderEmail, outsiderPass := uniqueEmail(), "pass456"
	register(t, s, outsiderEmail, uniqueName(), outsiderPass)
	outsiderToken := login(t, s, outsiderEmail, outsiderPass)

	addResp := s.POST(t, "/api/v1/tasks/"+taskID+"/comments",
		map[string]any{"body": "I shouldn't be here"}, outsiderToken)
	defer addResp.Body.Close()
	assert.Equal(t, http.StatusForbidden, addResp.StatusCode)

	listResp := s.GET(t, "/api/v1/tasks/"+taskID+"/comments", outsiderToken)
	defer listResp.Body.Close()
	assert.Equal(t, http.StatusForbidden, listResp.StatusCode)
}
