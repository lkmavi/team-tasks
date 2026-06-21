// seed populates the API with a realistic set of test data:
// 3 users, 2 teams, cross-membership, 6 tasks with varied statuses/priorities, and comments.
// Usage: go run ./scripts/seed [--base-url http://localhost:8080/api/v1]
package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"flag"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

const (
	keyStatus = "status"
	keyDone   = "done"
)

var baseURL string

func main() {
	flag.StringVar(&baseURL, "base-url", "http://localhost:8080/api/v1", "API base URL")
	flag.Parse()

	log.Printf("seeding %s", baseURL)

	register("alice@example.com", "Alice", "secret123")
	register("bob@example.com", "Bob", "secret123")
	register("carol@example.com", "Carol", "secret123")

	aliceToken := login("alice@example.com", "secret123")
	bobToken := login("bob@example.com", "secret123")
	carolToken := login("carol@example.com", "secret123")

	bobID := jwtSub(bobToken)
	carolID := jwtSub(carolToken)

	backendID := createTeam(aliceToken, "Backend Squad")
	frontendID := createTeam(bobToken, "Frontend Guild")

	invite(aliceToken, backendID, bobID)
	invite(aliceToken, backendID, carolID)
	invite(bobToken, frontendID, carolID)

	t1 := createTask(aliceToken, backendID, "Set up CI pipeline", "Configure GitHub Actions for lint, test, build", "high", "2026-07-15")
	t2 := createTask(aliceToken, backendID, "Design auth schema", "Users, sessions, refresh tokens", "medium", "2026-07-10")
	t3 := createTask(bobToken, backendID, "Write API docs", "Document all v1 endpoints in OpenAPI", "low", "2026-08-01")

	updateTask(aliceToken, t1, map[string]any{keyStatus: "in_progress"})
	updateTask(aliceToken, t2, map[string]any{keyStatus: keyDone})

	addComment(aliceToken, t1, "Pipeline draft is ready, needs review.")
	addComment(bobToken, t1, "LGTM, merging once tests pass.")
	addComment(aliceToken, t2, "Went with UUIDs for user IDs.")

	t4 := createTask(bobToken, frontendID, "Scaffold React app", "Vite + TypeScript + Tailwind", "high", "2026-07-05")
	t5 := createTask(bobToken, frontendID, "Login page UI", "Email + password form, error states", "medium", "2026-07-12")
	t6 := createTask(carolToken, frontendID, "Dark mode support", "", "low", "2026-08-15")

	updateTask(bobToken, t4, map[string]any{keyStatus: keyDone})
	updateTask(bobToken, t5, map[string]any{keyStatus: "in_progress", "priority": "high"})

	addComment(carolToken, t5, "Designs are in Figma, link in Notion.")
	addComment(bobToken, t6, "Can we use prefers-color-scheme for this?")

	log.Println("seed complete")
	log.Printf("  alice token  : %s", aliceToken)
	log.Printf("  bob token    : %s", bobToken)
	log.Printf("  carol token  : %s", carolToken)
	log.Printf("  backend team : %s", backendID)
	log.Printf("  frontend team: %s", frontendID)
	log.Printf("  tasks        : %s %s %s %s %s %s", t1, t2, t3, t4, t5, t6)
}

func register(email, name, password string) {
	code := do("POST", "/register", "", map[string]any{"email": email, "name": name, "password": password})
	if code != 204 && code != 201 && code != 200 && code != 409 {
		log.Fatalf("register %s: unexpected status %d", email, code)
	}
	log.Printf("register %s → %d", email, code)
}

func login(email, password string) string {
	var out struct {
		Token string `json:"token"`
	}
	doJSON("POST", "/login", "", map[string]any{"email": email, "password": password}, &out)
	log.Printf("login %s → token …%s", email, tail(out.Token, 8))
	return out.Token
}

func createTeam(token, name string) string {
	var out struct {
		ID string `json:"id"`
	}
	doJSON("POST", "/teams", token, map[string]any{"name": name}, &out)
	log.Printf("createTeam %q → %s", name, out.ID)
	return out.ID
}

func invite(token, teamID, targetUserID string) {
	code := do("POST", "/teams/"+teamID+"/invite", token, map[string]any{"user_id": targetUserID})
	log.Printf("invite user %s to team %s → %d", targetUserID, teamID, code)
}

func createTask(token, teamID, title, desc, priority, dueDate string) string {
	body := map[string]any{
		"team_id":  teamID,
		"title":    title,
		"priority": priority,
		"due_date": dueDate,
	}
	if desc != "" {
		body["description"] = desc
	}
	var out struct {
		ID string `json:"id"`
	}
	doJSON("POST", "/tasks", token, body, &out)
	log.Printf("createTask %q → %s", title, out.ID)
	return out.ID
}

func updateTask(token, taskID string, fields map[string]any) {
	code := do("PUT", "/tasks/"+taskID, token, fields)
	log.Printf("updateTask %s → %d", taskID, code)
}

func addComment(token, taskID, body string) {
	code := do("POST", "/tasks/"+taskID+"/comments", token, map[string]any{"body": body})
	log.Printf("addComment on %s → %d", taskID, code)
}

func do(method, path, token string, body any) int {
	resp := execRequest(method, path, token, body)
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	return resp.StatusCode
}

func doJSON(method, path, token string, body, out any) {
	resp := execRequest(method, path, token, body)
	raw, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode >= 300 {
		log.Fatalf("%s %s → %d: %s", method, path, resp.StatusCode, raw)
	}
	if out != nil {
		if err := json.Unmarshal(raw, out); err != nil {
			log.Fatalf("decode response from %s %s: %v\nbody: %s", method, path, err, raw)
		}
	}
}

func execRequest(method, path, token string, body any) *http.Response {
	var r io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, baseURL+path, r)
	if err != nil {
		log.Fatalf("build request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		log.Fatalf("%s %s: %v", method, path, err)
	}
	return resp
}

// jwtSub extracts the "sub" claim from a JWT without verifying the signature.
func jwtSub(token string) string {
	parts := strings.SplitN(token, ".", 3)
	if len(parts) != 3 {
		return ""
	}
	b, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return ""
	}
	var claims map[string]any
	if err = json.Unmarshal(b, &claims); err != nil {
		return ""
	}
	sub, _ := claims["sub"].(string)
	return sub
}

func tail(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}
