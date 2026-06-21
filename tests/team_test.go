//go:build integration

package tests

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/lkmavi/team-tasks/tests/suite"
)

func TestTeam_Create_OK(t *testing.T) {
	_, s := suite.New(t)

	email, pass := uniqueEmail(), "pass123"
	register(t, s, email, uniqueName(), pass)
	token := login(t, s, email, pass)

	teamName := "Backend-" + uuid.New().String()[:6]
	resp := s.POST(t, "/api/v1/teams", map[string]any{"name": teamName}, token)
	defer resp.Body.Close()

	var body struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	suite.Decode(t, resp, &body)

	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	assert.NotEmpty(t, body.ID)
	assert.Equal(t, teamName, body.Name)
}

func TestTeam_Create_Unauthorized(t *testing.T) {
	_, s := suite.New(t)

	resp := s.POST(t, "/api/v1/teams", map[string]any{"name": "secret"}, "")
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestTeam_Create_EmptyBody(t *testing.T) {
	_, s := suite.New(t)

	email, pass := uniqueEmail(), "pass123"
	register(t, s, email, uniqueName(), pass)
	token := login(t, s, email, pass)

	resp := s.POST(t, "/api/v1/teams", nil, token)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestTeam_List_Empty(t *testing.T) {
	_, s := suite.New(t)

	// freshly registered user has no teams
	email, pass := uniqueEmail(), "pass123"
	register(t, s, email, uniqueName(), pass)
	token := login(t, s, email, pass)

	resp := s.GET(t, "/api/v1/teams", token)
	defer resp.Body.Close()

	var body struct {
		Teams []map[string]any `json:"teams"`
	}
	suite.Decode(t, resp, &body)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Empty(t, body.Teams)
}

func TestTeam_List_ShowsCreatedTeam(t *testing.T) {
	_, s := suite.New(t)

	email, pass := uniqueEmail(), "pass123"
	register(t, s, email, uniqueName(), pass)
	token := login(t, s, email, pass)

	teamID := createTeam(t, s, "MyTeam", token)

	resp := s.GET(t, "/api/v1/teams", token)
	defer resp.Body.Close()

	var body struct {
		Teams []map[string]any `json:"teams"`
	}
	suite.Decode(t, resp, &body)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Len(t, body.Teams, 1)
	assert.Equal(t, teamID, body.Teams[0]["id"])
}

func TestTeam_List_Unauthorized(t *testing.T) {
	_, s := suite.New(t)

	resp := s.GET(t, "/api/v1/teams", "")
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestTeam_Invite_OK(t *testing.T) {
	_, s := suite.New(t)

	ownerEmail, ownerPass := uniqueEmail(), "ownerpass"
	register(t, s, ownerEmail, "Owner", ownerPass)
	ownerToken := login(t, s, ownerEmail, ownerPass)

	guestEmail, guestPass := uniqueEmail(), "guestpass"
	register(t, s, guestEmail, "Guest", guestPass)
	guestToken := login(t, s, guestEmail, guestPass)

	teamID := createTeam(t, s, "InviteTeam", ownerToken)
	guestUserID := userIDFromDB(t, s, guestEmail)

	inviteResp := s.POST(t, "/api/v1/teams/"+teamID+"/invite",
		map[string]any{"user_id": guestUserID}, ownerToken)
	defer inviteResp.Body.Close()
	assert.Equal(t, http.StatusOK, inviteResp.StatusCode)

	// invited user now sees the team in their list
	listResp := s.GET(t, "/api/v1/teams", guestToken)
	defer listResp.Body.Close()

	var body struct {
		Teams []map[string]any `json:"teams"`
	}
	suite.Decode(t, listResp, &body)

	found := false
	for _, tm := range body.Teams {
		if tm["id"] == teamID {
			found = true
		}
	}
	assert.True(t, found, "invited user must see the team")
}

func TestTeam_Invite_Unauthorized(t *testing.T) {
	_, s := suite.New(t)

	resp := s.POST(t, "/api/v1/teams/"+uuid.New().String()+"/invite",
		map[string]any{"user_id": uuid.New().String()}, "")
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

// TestTeam_Invite_AsMember_Forbidden verifies that a regular member (not owner/admin)
// cannot invite others to the team.
func TestTeam_Invite_AsMember_Forbidden(t *testing.T) {
	_, s := suite.New(t)

	ownerEmail, ownerPass := uniqueEmail(), "ownerpass"
	register(t, s, ownerEmail, "Owner", ownerPass)
	ownerToken := login(t, s, ownerEmail, ownerPass)

	memberEmail, memberPass := uniqueEmail(), "memberpass"
	register(t, s, memberEmail, "Member", memberPass)
	memberToken := login(t, s, memberEmail, memberPass)

	outsiderEmail := uniqueEmail()
	register(t, s, outsiderEmail, "Outsider", "outsiderpass")

	teamID := createTeam(t, s, "PermTeam", ownerToken)

	// invite member into the team as a plain member
	memberUserID := userIDFromDB(t, s, memberEmail)
	invOK := s.POST(t, "/api/v1/teams/"+teamID+"/invite",
		map[string]any{"user_id": memberUserID}, ownerToken)
	invOK.Body.Close()

	// member now tries to invite the outsider — must fail
	outsiderUserID := userIDFromDB(t, s, outsiderEmail)
	resp := s.POST(t, "/api/v1/teams/"+teamID+"/invite",
		map[string]any{"user_id": outsiderUserID}, memberToken)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

// TestTeam_Invite_UserNotFound verifies 404 when the target user does not exist.
func TestTeam_Invite_UserNotFound(t *testing.T) {
	_, s := suite.New(t)

	ownerEmail, ownerPass := uniqueEmail(), "ownerpass"
	register(t, s, ownerEmail, "Owner", ownerPass)
	ownerToken := login(t, s, ownerEmail, ownerPass)

	teamID := createTeam(t, s, "TeamX", ownerToken)

	resp := s.POST(t, "/api/v1/teams/"+teamID+"/invite",
		map[string]any{"user_id": uuid.New().String()}, ownerToken)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}
