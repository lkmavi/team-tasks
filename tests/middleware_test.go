//go:build integration

package tests

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lkmavi/team-tasks/tests/suite"
)

func TestJWT_NonBearerHeader_Unauthorized(t *testing.T) {
	_, s := suite.New(t)

	req, err := http.NewRequest(http.MethodGet, s.URL("/api/v1/tasks"), nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")

	resp, err := s.Client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestJWT_InvalidToken_Unauthorized(t *testing.T) {
	_, s := suite.New(t)

	req, err := http.NewRequest(http.MethodGet, s.URL("/api/v1/tasks"), nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer not.a.valid.jwt.token")

	resp, err := s.Client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}
