package client

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/quay/quay-mcp-server/internal/types"
)

const sampleSwagger = `{
  "swagger": "2.0",
  "info": {"title": "Quay", "version": "1.0"},
  "host": "quay.io",
  "basePath": "/api/v1",
  "schemes": ["https"],
  "paths": {
    "/repository": {
      "get": {
        "summary": "List repositories",
        "operationId": "listRepos",
        "tags": ["repository"],
        "responses": {"200": {"description": "ok"}}
      }
    },
    "/repository/{repository}/tag/": {
      "get": {
        "summary": "List tags",
        "operationId": "listRepoTags",
        "tags": ["tag"],
        "parameters": [
          {"name": "repository", "in": "path", "required": true, "type": "string"}
        ],
        "responses": {"200": {"description": "ok"}}
      }
    },
    "/repository/{repository}/manifest/{manifestref}/security": {
      "get": {
        "summary": "Get security scan",
        "operationId": "getRepoManifestSecurity",
        "tags": ["secscan"],
        "parameters": [
          {"name": "repository", "in": "path", "required": true, "type": "string"},
          {"name": "manifestref", "in": "path", "required": true, "type": "string"}
        ],
        "responses": {"200": {"description": "ok"}}
      }
    },
    "/organization/{orgname}/robots": {
      "get": {
        "summary": "List org robots",
        "operationId": "getOrgRobots",
        "tags": ["robot"],
        "parameters": [
          {"name": "orgname", "in": "path", "required": true, "type": "string"}
        ],
        "responses": {"200": {"description": "ok"}}
      }
    },
    "/user": {
      "get": {
        "summary": "Get current user",
        "operationId": "getLoggedUser",
        "tags": ["user"],
        "responses": {"200": {"description": "ok"}}
      }
    }
  }
}`

func discoveryServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/discovery" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(sampleSwagger))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
}

func TestNewQuayClientTrimsURL(t *testing.T) {
	c := NewQuayClient("https://quay.io/", "")
	if c.GetRegistryURL() != "https://quay.io" {
		t.Errorf("expected trimmed registry URL, got %q", c.GetRegistryURL())
	}
}

func TestFetchSwaggerSpecAndDiscoverEndpoints(t *testing.T) {
	mockServer := discoveryServer(t)
	defer mockServer.Close()

	c := NewQuayClient(mockServer.URL, "")
	if err := c.FetchSwaggerSpec(); err != nil {
		t.Fatalf("FetchSwaggerSpec: %v", err)
	}

	model := c.GetModel()
	if model == nil {
		t.Fatal("expected model to be loaded")
	}
	if model.Model.Host != "quay.io" {
		t.Errorf("expected host quay.io, got %q", model.Model.Host)
	}
	if model.Model.BasePath != "/api/v1" {
		t.Errorf("expected basePath /api/v1, got %q", model.Model.BasePath)
	}

	c.DiscoverEndpoints()
	endpoints := c.GetEndpoints()

	ops := map[string]bool{}
	for _, ep := range endpoints {
		ops[ep.OperationID] = true
	}

	for _, want := range []string{"listRepos", "listRepoTags", "getOrgRobots"} {
		if !ops[want] {
			t.Errorf("expected endpoint %q to be discovered", want)
		}
	}
	if ops["getRepoManifestSecurity"] {
		t.Error("secscan endpoints should be filtered out")
	}
	if ops["getLoggedUser"] {
		t.Error("user endpoints should be filtered out")
	}
}

func TestGenerateTools(t *testing.T) {
	mockServer := discoveryServer(t)
	defer mockServer.Close()

	c := NewQuayClient(mockServer.URL, "")
	if err := c.FetchSwaggerSpec(); err != nil {
		t.Fatalf("FetchSwaggerSpec: %v", err)
	}

	tools := c.GenerateTools()
	names := map[string]bool{}
	for _, tool := range tools {
		names[tool.Name] = true
		if !strings.HasPrefix(tool.Name, "quay_") {
			t.Errorf("tool name %q should start with quay_", tool.Name)
		}
	}

	for _, want := range []string{"quay_listRepos", "quay_listRepoTags", "quay_getOrgRobots"} {
		if !names[want] {
			t.Errorf("expected tool %q", want)
		}
	}
	if names["quay_getRepoManifestSecurity"] {
		t.Error("secscan tools should be filtered out")
	}
	if names["quay_getLoggedUser"] {
		t.Error("user tools should be filtered out")
	}
}

func TestExtractPathParameters(t *testing.T) {
	c := NewQuayClient("https://quay.io", "")

	tests := []struct {
		resourceURI  string
		pathTemplate string
		expected     map[string]string
	}{
		{
			resourceURI:  "quay://api/v1/repository/myorg/myrepo",
			pathTemplate: "/api/v1/repository/{namespace}/{repository}",
			expected:     map[string]string{"namespace": "myorg", "repository": "myrepo"},
		},
		{
			resourceURI:  "quay://api/v1/user/john",
			pathTemplate: "/api/v1/user/{username}",
			expected:     map[string]string{"username": "john"},
		},
		{
			resourceURI:  "quay://api/v1/health",
			pathTemplate: "/api/v1/health",
			expected:     map[string]string{},
		},
	}

	for _, test := range tests {
		result := c.extractPathParameters(test.resourceURI, test.pathTemplate)
		if len(result) != len(test.expected) {
			t.Errorf("Expected %d parameters, got %d for URI %s (%v)", len(test.expected), len(result), test.resourceURI, result)
			continue
		}
		for key, expectedValue := range test.expected {
			if actualValue, exists := result[key]; !exists || actualValue != expectedValue {
				t.Errorf("Expected parameter %s=%s, got %s for URI %s", key, expectedValue, actualValue, test.resourceURI)
			}
		}
	}
}

func TestBuildAPIURL(t *testing.T) {
	c := NewQuayClient("https://quay.io", "")
	endpoint := &types.EndpointInfo{
		Method: "GET",
		Path:   "/api/v1/repository/{namespace}/{repository}",
	}

	url, err := c.BuildAPIURL(endpoint, "quay://api/v1/repository/myorg/myrepo")
	if err != nil {
		t.Fatalf("BuildAPIURL: %v", err)
	}

	expected := "https://quay.io/api/v1/repository/myorg/myrepo"
	if url != expected {
		t.Errorf("Expected URL %q, got %q", expected, url)
	}
}

func TestBuildAPIURLWithParams(t *testing.T) {
	c := NewQuayClient("https://quay.io", "")
	endpoint := &types.EndpointInfo{
		Method: "GET",
		Path:   "/api/v1/repository/{repository}/tag/",
	}

	url, err := c.BuildAPIURLWithParams(endpoint, map[string]interface{}{
		"repository":  "it-rat/hermes-agent",
		"specificTag": "latest",
	})
	if err != nil {
		t.Fatalf("BuildAPIURLWithParams: %v", err)
	}

	expected := "https://quay.io/api/v1/repository/it-rat/hermes-agent/tag/?specificTag=latest"
	if url != expected {
		t.Errorf("Expected URL %q, got %q", expected, url)
	}
}

func TestHasPathParameters(t *testing.T) {
	c := NewQuayClient("https://quay.io", "")
	if !c.HasPathParameters("/api/v1/repository/{repository}") {
		t.Error("expected path parameters")
	}
	if c.HasPathParameters("/api/v1/repository") {
		t.Error("did not expect path parameters")
	}
}

func TestMakeAPICall(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"message": "success"}`))
	}))
	defer mockServer.Close()

	c := NewQuayClient(mockServer.URL, "")
	endpoint := &types.EndpointInfo{
		Method: "GET",
		Path:   "/test",
	}

	data, err := c.MakeAPICall(endpoint, "quay://test")
	if err != nil {
		t.Fatalf("MakeAPICall: %v", err)
	}

	expected := `{"message": "success"}`
	if string(data) != expected {
		t.Errorf("Expected response %q, got %q", expected, string(data))
	}
}

func TestOAuthTokenInAPICall(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error": "unauthorized"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"message": "authenticated"}`))
	}))
	defer mockServer.Close()

	c := NewQuayClient(mockServer.URL, "test-token")
	endpoint := &types.EndpointInfo{
		Method: "GET",
		Path:   "/test",
	}

	data, err := c.MakeAPICall(endpoint, "quay://test")
	if err != nil {
		t.Fatalf("MakeAPICall: %v", err)
	}

	expected := `{"message": "authenticated"}`
	if string(data) != expected {
		t.Errorf("Expected response %q, got %q", expected, string(data))
	}
}
