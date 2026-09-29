package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"fluxa-api/internal/bootstrap"
	"fluxa-api/internal/config"

	"go.uber.org/zap"
)

func TestLoginTokenCanAccessProjects(t *testing.T) {
	t.Setenv("BOOTSTRAP_ADMIN_PASSWORD", "test-password")
	cfg := config.Load()
	cfg.DBDriver = "sqlite"
	cfg.SQLitePath = filepath.Join(t.TempDir(), "fluxa.db")
	cfg.AutoMigrate = true
	cfg.JWTSecret = "test-jwt-secret"

	container, err := bootstrap.New(cfg, zap.NewNop())
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	router := NewRouter(container)

	login := httptest.NewRecorder()
	loginRequest := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login",
		bytes.NewBufferString(`{"user_name":"admin","password":"test-password"}`))
	loginRequest.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(login, loginRequest)
	if login.Code != http.StatusOK {
		t.Fatalf("login status = %d, body = %s", login.Code, login.Body.String())
	}

	var response struct {
		Data struct {
			AccessToken string `json:"access_token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(login.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode login response: %v", err)
	}
	if response.Data.AccessToken == "" {
		t.Fatal("login response did not contain an access token")
	}

	projects := httptest.NewRecorder()
	projectsRequest := httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
	projectsRequest.Header.Set("Authorization", "Bearer "+response.Data.AccessToken)
	router.ServeHTTP(projects, projectsRequest)
	if projects.Code != http.StatusOK {
		t.Fatalf("projects status = %d, body = %s", projects.Code, projects.Body.String())
	}
}

func TestPasswordResetRevokesExistingToken(t *testing.T) {
	t.Setenv("BOOTSTRAP_ADMIN_PASSWORD", "test-password")
	cfg := config.Load()
	cfg.DBDriver = "sqlite"
	cfg.SQLitePath = filepath.Join(t.TempDir(), "fluxa.db")
	cfg.AutoMigrate = true
	cfg.JWTSecret = "test-jwt-secret"
	container, err := bootstrap.New(cfg, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	router := NewRouter(container)
	token := loginWithCredentialsForTest(t, router, "admin", "test-password")
	reset := requestForTest(t, router, token, http.MethodPut, "/api/v1/users/1/password", `{"password":"replacement-password"}`)
	if reset.Code != http.StatusOK {
		t.Fatalf("reset password: %d %s", reset.Code, reset.Body.String())
	}
	me := requestForTest(t, router, token, http.MethodGet, "/api/v1/auth/me", "")
	if me.Code != http.StatusUnauthorized {
		t.Fatalf("old token should be rejected, got %d %s", me.Code, me.Body.String())
	}
	_ = loginWithCredentialsForTest(t, router, "admin", "replacement-password")
}
