package gitlabclient

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestSearchAndGetProject(t *testing.T) {
	client := New("https://gitlab.test", "secret", time.Second)
	client.http.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("Private-Token") != "secret" {
			t.Fatalf("missing private token")
		}
		body := ""
		switch r.URL.Path {
		case "/api/v4/projects":
			if r.URL.Query().Get("search") != "checkout api" {
				t.Fatalf("unexpected search query: %s", r.URL.RawQuery)
			}
			body = `[{"id":42,"name":"checkout","path_with_namespace":"shop/checkout","web_url":"https://gitlab/shop/checkout","default_branch":"main"}]`
		case "/api/v4/projects/42":
			body = `{"id":42,"name":"checkout","path_with_namespace":"shop/checkout","web_url":"https://gitlab/shop/checkout","default_branch":"main"}`
		default:
			return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})
	projects, err := client.SearchProjects(context.Background(), " checkout api ")
	if err != nil || len(projects) != 1 || projects[0].ID != 42 {
		t.Fatalf("unexpected search result: %#v, %v", projects, err)
	}
	project, err := client.GetProject(context.Background(), 42)
	if err != nil || project.PathWithNamespace != "shop/checkout" {
		t.Fatalf("unexpected project: %#v, %v", project, err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) { return f(request) }

func TestEmptySearchDoesNotRequireConfiguration(t *testing.T) {
	projects, err := New("", "", time.Second).SearchProjects(context.Background(), " ")
	if err != nil || len(projects) != 0 {
		t.Fatalf("unexpected result: %#v, %v", projects, err)
	}
}
