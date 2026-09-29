package httpapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"fluxa-api/internal/bootstrap"
	"fluxa-api/internal/config"

	"go.uber.org/zap"
)

func TestProjectGitlabRepositoryLifecycle(t *testing.T) {
	oldTransport := http.DefaultTransport
	http.DefaultTransport = testRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("Private-Token") != "gitlab-secret" {
			t.Fatalf("missing GitLab token")
		}
		body := ""
		switch r.URL.Path {
		case "/api/v4/projects":
			body = `[{"id":42,"name":"checkout","path_with_namespace":"shop/checkout","web_url":"https://gitlab/shop/checkout","default_branch":"main"}]`
		case "/api/v4/projects/42", "/api/v4/projects/43", "/api/v4/projects/44":
			id := strings.TrimPrefix(r.URL.Path, "/api/v4/projects/")
			body = fmt.Sprintf(`{"id":%s,"name":"repo-%s","path_with_namespace":"shop/repo-%s","web_url":"https://gitlab/shop/repo-%s","default_branch":"main"}`, id, id, id, id)
		default:
			return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})
	t.Cleanup(func() { http.DefaultTransport = oldTransport })

	t.Setenv("BOOTSTRAP_ADMIN_PASSWORD", "test-password")
	cfg := config.Load()
	cfg.DBDriver = "sqlite"
	cfg.SQLitePath = filepath.Join(t.TempDir(), "fluxa.db")
	cfg.AutoMigrate = true
	cfg.JWTSecret = "test-jwt-secret"
	cfg.GitlabURL = "https://gitlab.test"
	cfg.GitlabToken = "gitlab-secret"
	container, err := bootstrap.New(cfg, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	router := NewRouter(container)
	token := loginForTest(t, router)

	created := requestForTest(t, router, token, http.MethodPost, "/api/v1/projects", `{"name":"Shop","key":"shop","description":""}`)
	if created.Code != http.StatusCreated {
		t.Fatalf("create project: %d %s", created.Code, created.Body.String())
	}
	var projectResponse struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &projectResponse); err != nil {
		t.Fatal(err)
	}
	projectID := projectResponse.Data.ID
	leadID := createUserForTest(t, router, token, "lead")
	developerID := createUserForTest(t, router, token, "developer")
	for _, member := range []struct {
		id   int64
		role string
	}{{leadID, "lead"}, {developerID, "developer"}} {
		response := requestForTest(t, router, token, http.MethodPost, fmt.Sprintf("/api/v1/projects/%s/members", projectID), fmt.Sprintf(`{"user_id":%d,"role_name":%q}`, member.id, member.role))
		if response.Code != http.StatusCreated {
			t.Fatalf("add %s: %d %s", member.role, response.Code, response.Body.String())
		}
	}

	search := requestForTest(t, router, token, http.MethodGet, "/api/v1/gitlab/projects?keyword=checkout", "")
	if search.Code != http.StatusOK {
		t.Fatalf("search: %d %s", search.Code, search.Body.String())
	}
	attachPath := fmt.Sprintf("/api/v1/projects/%s/gitlab-repositories", projectID)
	attached := requestForTest(t, router, token, http.MethodPost, attachPath, `{"gitlab_project_id":42}`)
	if attached.Code != http.StatusCreated {
		t.Fatalf("attach: %d %s", attached.Code, attached.Body.String())
	}
	leadToken := loginWithCredentialsForTest(t, router, "lead", "test-user-password")
	leadAttach := requestForTest(t, router, leadToken, http.MethodPost, attachPath, `{"gitlab_project_id":43}`)
	if leadAttach.Code != http.StatusCreated {
		t.Fatalf("lead should attach: %d %s", leadAttach.Code, leadAttach.Body.String())
	}
	developerToken := loginWithCredentialsForTest(t, router, "developer", "test-user-password")
	developerAttach := requestForTest(t, router, developerToken, http.MethodPost, attachPath, `{"gitlab_project_id":44}`)
	if developerAttach.Code != http.StatusForbidden {
		t.Fatalf("developer attach should be forbidden: %d %s", developerAttach.Code, developerAttach.Body.String())
	}
	for _, path := range []string{"/api/v1/workers", "/api/v1/workflows", "/api/v1/users"} {
		response := requestForTest(t, router, developerToken, http.MethodGet, path, "")
		if response.Code != http.StatusForbidden {
			t.Fatalf("normal user should not access %s: %d %s", path, response.Code, response.Body.String())
		}
	}
	workflowRead := requestForTest(t, router, developerToken, http.MethodGet, fmt.Sprintf("/api/v1/projects/%s/workflows/task", projectID), "")
	if workflowRead.Code != http.StatusOK {
		t.Fatalf("normal user still needs workflow runtime config: %d %s", workflowRead.Code, workflowRead.Body.String())
	}
	workflowUpdate := requestForTest(t, router, developerToken, http.MethodPut, fmt.Sprintf("/api/v1/projects/%s/workflows/task", projectID), `{}`)
	if workflowUpdate.Code != http.StatusForbidden {
		t.Fatalf("normal user should not update workflow: %d %s", workflowUpdate.Code, workflowUpdate.Body.String())
	}
	listed := requestForTest(t, router, token, http.MethodGet, attachPath, "")
	if listed.Code != http.StatusOK || !bytes.Contains(listed.Body.Bytes(), []byte(`"gitlab_project_id":42`)) {
		t.Fatalf("list: %d %s", listed.Code, listed.Body.String())
	}

	serviceBody := fmt.Sprintf(`{"project_id":%q,"gitlab_project_id":42,"service_key":"checkout","display_name":"Checkout","deploy_target":"portainer"}`, projectID)
	service := requestForTest(t, router, token, http.MethodPost, "/api/v1/project-services", serviceBody)
	if service.Code != http.StatusCreated {
		t.Fatalf("create service: %d %s", service.Code, service.Body.String())
	}
	var serviceResponse struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(service.Body.Bytes(), &serviceResponse); err != nil || serviceResponse.Data.ID == "" {
		t.Fatalf("decode service: %v %s", err, service.Body.String())
	}
	rotated := requestForTest(t, router, token, http.MethodPost, fmt.Sprintf("/api/v1/projects/%s/ci-token/rotate", projectID), `{}`)
	var tokenResponse struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	if rotated.Code != http.StatusOK || json.Unmarshal(rotated.Body.Bytes(), &tokenResponse) != nil || tokenResponse.Data.Token == "" {
		t.Fatalf("rotate CI token: %d %s", rotated.Code, rotated.Body.String())
	}
	reportBody := fmt.Sprintf(`{"project_service_id":%q,"commit_sha":"abc123","ref_type":"branch","ref":"feature/test","image_repository":"registry/shop","image_tag":"abc123","image_digest":"sha256:abc","pipeline_id":"10","idempotency_key":"pipeline-10"}`, serviceResponse.Data.ID)
	reported := requestForTest(t, router, tokenResponse.Data.Token, http.MethodPost, "/api/v1/ci/artifacts", reportBody)
	if reported.Code != http.StatusCreated || !bytes.Contains(reported.Body.Bytes(), []byte(`"deployment"`)) {
		t.Fatalf("report artifact and deploy dev: %d %s", reported.Code, reported.Body.String())
	}
	var artifactResponse struct {
		Data struct {
			Artifact struct {
				ID string `json:"id"`
			} `json:"artifact"`
		} `json:"data"`
	}
	if err := json.Unmarshal(reported.Body.Bytes(), &artifactResponse); err != nil || artifactResponse.Data.Artifact.ID == "" {
		t.Fatalf("decode artifact: %v", err)
	}
	reportedAgain := requestForTest(t, router, tokenResponse.Data.Token, http.MethodPost, "/api/v1/ci/artifacts", reportBody)
	if reportedAgain.Code != http.StatusCreated {
		t.Fatalf("idempotent report: %d %s", reportedAgain.Code, reportedAgain.Body.String())
	}
	taskCreated := requestForTest(t, router, token, http.MethodPost, "/api/v1/tasks", fmt.Sprintf(`{"project_id":%q,"title":"Checkout task"}`, projectID))
	var taskResponse struct {
		Data struct {
			ID      string `json:"id"`
			Creator string `json:"creator"`
		} `json:"data"`
	}
	if taskCreated.Code != http.StatusCreated || json.Unmarshal(taskCreated.Body.Bytes(), &taskResponse) != nil {
		t.Fatalf("create task: %d %s", taskCreated.Code, taskCreated.Body.String())
	}
	if taskResponse.Data.Creator != "admin" {
		t.Fatalf("task creator should be admin, got %q in %s", taskResponse.Data.Creator, taskCreated.Body.String())
	}
	taskList := requestForTest(t, router, token, http.MethodGet, fmt.Sprintf("/api/v1/tasks?project_id=%s", projectID), "")
	if taskList.Code != http.StatusOK || !bytes.Contains(taskList.Body.Bytes(), []byte(`"creator":"admin"`)) {
		t.Fatalf("list task creator: %d %s", taskList.Code, taskList.Body.String())
	}
	assigned := requestForTest(t, router, token, http.MethodPatch, "/api/v1/tasks/"+taskResponse.Data.ID+"/assignee", `{"assignee":"lead"}`)
	if assigned.Code != http.StatusOK || !bytes.Contains(assigned.Body.Bytes(), []byte(`"assignee":"lead"`)) {
		t.Fatalf("assign project member: %d %s", assigned.Code, assigned.Body.String())
	}
	invalidAssignee := requestForTest(t, router, token, http.MethodPatch, "/api/v1/tasks/"+taskResponse.Data.ID+"/assignee", `{"assignee":"not-a-member"}`)
	if invalidAssignee.Code != http.StatusBadRequest {
		t.Fatalf("assign non-member should fail: %d %s", invalidAssignee.Code, invalidAssignee.Body.String())
	}
	for _, status := range []string{"in_progress", "publishing"} {
		moved := requestForTest(t, router, token, http.MethodPatch, "/api/v1/tasks/"+taskResponse.Data.ID+"/status", fmt.Sprintf(`{"status":%q}`, status))
		if moved.Code != http.StatusOK {
			t.Fatalf("move task %s: %d %s", status, moved.Code, moved.Body.String())
		}
	}
	createRelease := func() *httptest.ResponseRecorder {
		body := fmt.Sprintf(`{"project_id":%q,"title":"Release","task_ids":[%q],"services":[{"project_service_id":%q,"artifact_id":%q}]}`, projectID, taskResponse.Data.ID, serviceResponse.Data.ID, artifactResponse.Data.Artifact.ID)
		return requestForTest(t, router, token, http.MethodPost, "/api/v1/releases", body)
	}
	createdRelease := createRelease()
	var releaseResponse struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if createdRelease.Code != http.StatusCreated || json.Unmarshal(createdRelease.Body.Bytes(), &releaseResponse) != nil {
		t.Fatalf("create release: %d %s", createdRelease.Code, createdRelease.Body.String())
	}
	if bytes.Contains(createdRelease.Body.Bytes(), []byte(`"environment"`)) {
		t.Fatalf("release response should not expose environment: %s", createdRelease.Body.String())
	}
	submitted := requestForTest(t, router, token, http.MethodPost, "/api/v1/releases/"+releaseResponse.Data.ID+"/submit", `{}`)
	if submitted.Code != http.StatusOK || !bytes.Contains(submitted.Body.Bytes(), []byte(`"status":"pending_approval"`)) {
		t.Fatalf("release should await approval before flow execution: %d %s", submitted.Code, submitted.Body.String())
	}
	approved := requestForTest(t, router, token, http.MethodPost, "/api/v1/releases/"+releaseResponse.Data.ID+"/approve", `{}`)
	if approved.Code != http.StatusOK || !bytes.Contains(approved.Body.Bytes(), []byte(`"status":"publishing"`)) {
		t.Fatalf("approved release should start execution: %d %s", approved.Code, approved.Body.String())
	}
	detach := requestForTest(t, router, token, http.MethodDelete, attachPath+"/42", "")
	if detach.Code != http.StatusConflict {
		t.Fatalf("expected dependency conflict: %d %s", detach.Code, detach.Body.String())
	}
}

type testRoundTripFunc func(*http.Request) (*http.Response, error)

func (f testRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func loginForTest(t *testing.T, router http.Handler) string {
	t.Helper()
	return loginWithCredentialsForTest(t, router, "admin", "test-password")
}

func loginWithCredentialsForTest(t *testing.T, router http.Handler, userName, password string) string {
	t.Helper()
	login := requestForTest(t, router, "", http.MethodPost, "/api/v1/auth/login", fmt.Sprintf(`{"user_name":%q,"password":%q}`, userName, password))
	var response struct {
		Data struct {
			AccessToken string `json:"access_token"`
		} `json:"data"`
	}
	if login.Code != http.StatusOK || json.Unmarshal(login.Body.Bytes(), &response) != nil || response.Data.AccessToken == "" {
		t.Fatalf("login failed: %d %s", login.Code, login.Body.String())
	}
	return response.Data.AccessToken
}

func createUserForTest(t *testing.T, router http.Handler, token, userName string) int64 {
	t.Helper()
	response := requestForTest(t, router, token, http.MethodPost, "/api/v1/users", fmt.Sprintf(`{"user_name":%q,"user_name_cn":%q,"user_email":%q,"user_type":0,"initial_password":"test-user-password"}`, userName, userName, userName+"@test.local"))
	var decoded struct {
		Data struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}
	if response.Code != http.StatusCreated || json.Unmarshal(response.Body.Bytes(), &decoded) != nil || decoded.Data.ID == 0 {
		t.Fatalf("create user %s failed: %d %s", userName, response.Code, response.Body.String())
	}
	return decoded.Data.ID
}

func requestForTest(t *testing.T, router http.Handler, token, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	router.ServeHTTP(recorder, request)
	return recorder
}
