//go:build integration

package tests

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/lkmavi/team-tasks/tests/suite"
)

func uniqueEmail() string { return fmt.Sprintf("user-%s@example.com", uuid.New().String()[:8]) }
func uniqueName() string  { return "User-" + uuid.New().String()[:8] }

// register creates a user, asserts 201, and fails the test on any other status.
func register(t *testing.T, s *suite.Suite, email, name, password string) {
	t.Helper()
	resp := s.POST(t, "/api/v1/register", map[string]any{
		"email": email, "name": name, "password": password,
	}, "")
	defer resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode, "register %s", email)
}

// login authenticates and returns the JWT token; fails the test if no token returned.
func login(t *testing.T, s *suite.Suite, email, password string) string {
	t.Helper()
	resp := s.POST(t, "/api/v1/login", map[string]any{
		"email": email, "password": password,
	}, "")
	defer resp.Body.Close()
	var body struct {
		Token string `json:"token"`
	}
	suite.Decode(t, resp, &body)
	require.NotEmpty(t, body.Token, "login %s must return JWT", email)
	return body.Token
}

// createTeam creates a team and returns its ID; fails the test if the request fails.
func createTeam(t *testing.T, s *suite.Suite, name, token string) string {
	t.Helper()
	resp := s.POST(t, "/api/v1/teams", map[string]any{"name": name}, token)
	defer resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode, "createTeam %s", name)
	var body struct {
		ID string `json:"id"`
	}
	suite.Decode(t, resp, &body)
	require.NotEmpty(t, body.ID, "createTeam must return an ID")
	return body.ID
}

// createTask creates a task in the given team and returns its ID.
func createTask(t *testing.T, s *suite.Suite, teamID, title, token string) string {
	t.Helper()
	resp := s.POST(t, "/api/v1/tasks", map[string]any{
		"team_id": teamID, "title": title,
	}, token)
	defer resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode, "createTask %q", title)
	var body struct {
		ID string `json:"id"`
	}
	suite.Decode(t, resp, &body)
	require.NotEmpty(t, body.ID, "createTask must return an ID")
	return body.ID
}

// userIDFromDB fetches the binary-UUID-formatted user ID from the database by email.
func userIDFromDB(t *testing.T, s *suite.Suite, email string) string {
	t.Helper()
	var id string
	require.NoError(t, s.DB.QueryRow(
		"SELECT BIN_TO_UUID(id) FROM users WHERE email = ?", email,
	).Scan(&id))
	return id
}
