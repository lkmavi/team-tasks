//go:build integration

package tests

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lkmavi/team-tasks/tests/suite"
)

func TestAnalytics_TeamSummaries_OK(t *testing.T) {
	_, s := suite.New(t)

	email, pass := uniqueEmail(), "pass123"
	register(t, s, email, uniqueName(), pass)
	token := login(t, s, email, pass)
	createTeam(t, s, "SummaryTeam", token)

	resp := s.GET(t, "/api/v1/analytics/team-summaries", token)
	defer resp.Body.Close()

	var summaries []map[string]any
	suite.Decode(t, resp, &summaries)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	require.NotNil(t, summaries, "response must be a JSON array, not null")
	require.Len(t, summaries, 1, "user belongs to exactly one team")

	s0 := summaries[0]
	assert.Equal(t, "SummaryTeam", s0["name"])
	assert.GreaterOrEqual(t, int(s0["member_count"].(float64)), 1)
}

func TestAnalytics_TeamSummaries_Unauthorized(t *testing.T) {
	_, s := suite.New(t)

	resp := s.GET(t, "/api/v1/analytics/team-summaries", "")
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestAnalytics_TopContributors_OK(t *testing.T) {
	_, s := suite.New(t)

	email, pass := uniqueEmail(), "pass123"
	register(t, s, email, uniqueName(), pass)
	token := login(t, s, email, pass)
	teamID := createTeam(t, s, "ContribTeam", token)

	createTask(t, s, teamID, "Task 1", token)
	createTask(t, s, teamID, "Task 2", token)

	resp := s.GET(t, "/api/v1/analytics/top-contributors", token)
	defer resp.Body.Close()

	var contributors []map[string]any
	suite.Decode(t, resp, &contributors)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	require.NotNil(t, contributors)
	require.NotEmpty(t, contributors, "owner created 2 tasks, must appear as top contributor")

	c0 := contributors[0]
	assert.EqualValues(t, 1, c0["rank_in_team"])
	assert.GreaterOrEqual(t, int(c0["task_count"].(float64)), 2)
}

func TestAnalytics_TopContributors_Unauthorized(t *testing.T) {
	_, s := suite.New(t)

	resp := s.GET(t, "/api/v1/analytics/top-contributors", "")
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

// TestAnalytics_OrphanTasks_Empty verifies the endpoint returns a valid empty array
// when no assignee has been removed from their team.
func TestAnalytics_OrphanTasks_Empty(t *testing.T) {
	_, s := suite.New(t)

	email, pass := uniqueEmail(), "pass123"
	register(t, s, email, uniqueName(), pass)
	token := login(t, s, email, pass)

	resp := s.GET(t, "/api/v1/analytics/orphan-tasks", token)
	defer resp.Body.Close()

	var orphans []map[string]any
	suite.Decode(t, resp, &orphans)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.NotNil(t, orphans, "must return [] not null")
}

// TestAnalytics_OrphanTasks_OK seeds an orphan task directly in the DB
// (assign task to a user who is then removed from team_members) and verifies
// the endpoint surfaces it.
func TestAnalytics_OrphanTasks_OK(t *testing.T) {
	_, s := suite.New(t)

	ownerEmail, ownerPass := uniqueEmail(), "pass123"
	register(t, s, ownerEmail, uniqueName(), ownerPass)
	ownerToken := login(t, s, ownerEmail, ownerPass)

	memberEmail, memberPass := uniqueEmail(), "pass456"
	register(t, s, memberEmail, uniqueName(), memberPass)

	teamID := createTeam(t, s, "OrphanTeam", ownerToken)
	memberUserID := userIDFromDB(t, s, memberEmail)

	// invite member into team
	invResp := s.POST(t, "/api/v1/teams/"+teamID+"/invite",
		map[string]any{"user_id": memberUserID}, ownerToken)
	invResp.Body.Close()
	require.Equal(t, http.StatusOK, invResp.StatusCode)

	// assign a task to member
	taskID := createTask(t, s, teamID, "Orphan candidate", ownerToken)
	updResp := s.PUT(t, "/api/v1/tasks/"+taskID,
		map[string]any{"assignee_id": memberUserID}, ownerToken)
	updResp.Body.Close()
	require.Equal(t, http.StatusOK, updResp.StatusCode)

	// remove member from team directly in DB to create orphan state
	_, err := s.DB.Exec(
		"DELETE FROM team_members WHERE team_id = UUID_TO_BIN(?) AND user_id = UUID_TO_BIN(?)",
		teamID, memberUserID,
	)
	require.NoError(t, err, "remove member from team to produce orphan task")

	resp := s.GET(t, "/api/v1/analytics/orphan-tasks", ownerToken)
	defer resp.Body.Close()

	var orphans []map[string]any
	suite.Decode(t, resp, &orphans)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	require.NotEmpty(t, orphans, "removed assignee must produce an orphan task")
	assert.Equal(t, taskID, orphans[0]["id"])
}

func TestAnalytics_OrphanTasks_Unauthorized(t *testing.T) {
	_, s := suite.New(t)

	resp := s.GET(t, "/api/v1/analytics/orphan-tasks", "")
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}
