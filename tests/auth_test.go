//go:build integration

package tests

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/lkmavi/team-tasks/tests/suite"
)

func TestAuth_Register_OK(t *testing.T) {
	_, s := suite.New(t)

	resp := s.POST(t, "/api/v1/register", map[string]any{
		"email": uniqueEmail(), "name": uniqueName(), "password": "StrongPass1!",
	}, "")
	defer resp.Body.Close()

	assert.Equal(t, http.StatusCreated, resp.StatusCode)
}

func TestAuth_Register_DuplicateEmail(t *testing.T) {
	_, s := suite.New(t)

	email := uniqueEmail()
	register(t, s, email, uniqueName(), "pass123")

	resp := s.POST(t, "/api/v1/register", map[string]any{
		"email": email, "name": uniqueName(), "password": "pass456",
	}, "")
	defer resp.Body.Close()

	assert.Equal(t, http.StatusConflict, resp.StatusCode)
}

func TestAuth_Register_EmptyBody(t *testing.T) {
	_, s := suite.New(t)

	resp := s.POST(t, "/api/v1/register", nil, "")
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestAuth_Login_OK(t *testing.T) {
	_, s := suite.New(t)

	email, pass := uniqueEmail(), "MyPass99!"
	register(t, s, email, uniqueName(), pass)

	resp := s.POST(t, "/api/v1/login", map[string]any{
		"email": email, "password": pass,
	}, "")
	defer resp.Body.Close()

	var body struct {
		Token string `json:"token"`
	}
	suite.Decode(t, resp, &body)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.NotEmpty(t, body.Token)
}

func TestAuth_Login_WrongPassword(t *testing.T) {
	_, s := suite.New(t)

	email := uniqueEmail()
	register(t, s, email, uniqueName(), "correct")

	resp := s.POST(t, "/api/v1/login", map[string]any{
		"email": email, "password": "wrong",
	}, "")
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestAuth_Login_UnknownEmail(t *testing.T) {
	_, s := suite.New(t)

	resp := s.POST(t, "/api/v1/login", map[string]any{
		"email": uniqueEmail(), "password": "irrelevant",
	}, "")
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestAuth_Login_EmptyBody(t *testing.T) {
	_, s := suite.New(t)

	resp := s.POST(t, "/api/v1/login", nil, "")
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}
