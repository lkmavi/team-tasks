package main

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func withServer(t *testing.T, handler http.HandlerFunc) {
	t.Helper()
	srv := httptest.NewServer(handler)
	orig := baseURL
	baseURL = srv.URL
	t.Cleanup(func() {
		srv.Close()
		baseURL = orig
	})
}

func jsonBody(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

func TestTail_LongerThanN(t *testing.T) {
	assert.Equal(t, "world", tail("hello world", 5))
}

func TestTail_ShorterThanN(t *testing.T) {
	assert.Equal(t, "hi", tail("hi", 10))
}

func TestJwtSub_Valid(t *testing.T) {
	payload := base64.RawURLEncoding.EncodeToString(jsonBody(map[string]any{"sub": "user-abc"}))
	assert.Equal(t, "user-abc", jwtSub("hdr."+payload+".sig"))
}

func TestJwtSub_NoSubField(t *testing.T) {
	payload := base64.RawURLEncoding.EncodeToString(jsonBody(map[string]any{"exp": 1}))
	assert.Equal(t, "", jwtSub("hdr."+payload+".sig"))
}

func TestJwtSub_TooFewParts(t *testing.T) {
	assert.Equal(t, "", jwtSub("only.two"))
}

func TestJwtSub_InvalidBase64(t *testing.T) {
	assert.Equal(t, "", jwtSub("hdr.!!!.sig"))
}

func TestJwtSub_InvalidJSON(t *testing.T) {
	payload := base64.RawURLEncoding.EncodeToString([]byte("not-json"))
	assert.Equal(t, "", jwtSub("hdr."+payload+".sig"))
}

func TestDo_NoBody(t *testing.T) {
	withServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	assert.Equal(t, http.StatusOK, do("GET", "", "", nil))
}

func TestDo_WithBodyAndToken(t *testing.T) {
	withServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "Bearer tok", r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusCreated)
	})
	assert.Equal(t, http.StatusCreated, do("POST", "", "tok", map[string]any{"k": "v"}))
}

func TestDoJSON_SuccessWithOutput(t *testing.T) {
	withServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(jsonBody(map[string]string{"id": "xyz"}))
	})
	var out struct {
		ID string `json:"id"`
	}
	doJSON("GET", "", "", nil, &out)
	assert.Equal(t, "xyz", out.ID)
}

func TestDoJSON_NilOutput(t *testing.T) {
	withServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	doJSON("DELETE", "", "", nil, nil)
}

func TestRegister_NoContent(t *testing.T) {
	withServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	register("a@b.com", "A", "pass")
}

func TestRegister_Conflict(t *testing.T) {
	withServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusConflict)
	})
	register("a@b.com", "A", "pass")
}

func TestLogin_ReturnsToken(t *testing.T) {
	withServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(jsonBody(map[string]string{"token": "jwt.tok.sig"}))
	})
	assert.Equal(t, "jwt.tok.sig", login("a@b.com", "pass"))
}

func TestCreateTeam_ReturnsID(t *testing.T) {
	withServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(jsonBody(map[string]string{"id": "team-1"}))
	})
	assert.Equal(t, "team-1", createTeam("tok", "Alpha"))
}

func TestInvite_OK(t *testing.T) {
	withServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	invite("tok", "team-1", "user-1")
}

func TestCreateTask_WithDescription(t *testing.T) {
	withServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(jsonBody(map[string]string{"id": "task-1"}))
	})
	assert.Equal(t, "task-1", createTask("tok", "team-1", "Title", "Desc", "high", "2026-07-01"))
}

func TestCreateTask_WithoutDescription(t *testing.T) {
	withServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(jsonBody(map[string]string{"id": "task-2"}))
	})
	assert.Equal(t, "task-2", createTask("tok", "team-1", "Title", "", "low", "2026-07-01"))
}

func TestUpdateTask_OK(t *testing.T) {
	withServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	updateTask("tok", "task-1", map[string]any{keyStatus: "done"})
}

func TestAddComment_OK(t *testing.T) {
	withServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})
	addComment("tok", "task-1", "looks good")
}
