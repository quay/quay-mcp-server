package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchSwaggerSpec(t *testing.T) {
	// Create a mock server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/discovery" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{
				"host": "quay.io",
				"basePath": "/api/v1",
				"schemes": ["https"],
				"paths": {
					"/repository": {
						"get": {
							"summary": "List repositories",
							"operationId": "listRepos",
							"tags": ["repository"]
						}
					}
				}
			}`))
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer mockServer.Close()

	client := NewQuayClient(mockServer.URL, "")
	err := client.FetchSwaggerSpec()

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	spec := client.GetSpec()
	if spec == nil {
		t.Fatal("Expected spec to be loaded")
	}

	if spec.Host != "quay.io" {
		t.Errorf("Expected host 'quay.io', got '%s'", spec.Host)
	}

	if len(spec.Paths) != 1 {
		t.Errorf("Expected 1 path, got %d", len(spec.Paths))
	}
}

func TestGenerateResourcesAndTemplates(t *testing.T) {
	server := NewQuayMCPServer("https://quay.io", "")

	// Mock swagger spec with both parameterized and non-parameterized endpoints
	server.quayClient.spec = &SwaggerSpec{
		Paths: map[string]PathDetails{
			"/api/v1/user": {
				Get: &OperationDetails{
					Summary:     "Get current user",
					Description: "Returns information about the current user",
					Tags:        []string{"user"},
					OperationID: "getCurrentUser",
				},
			},
			"/api/v1/repository/{namespace}/{repository}": {
				Get: &OperationDetails{
					Summary:     "Get repository",
					Description: "Get information about a repository",
					Tags:        []string{"repository"},
					OperationID: "getRepository",
				},
			},
			"/api/v1/health": {
				Get: &OperationDetails{
					Summary:     "Health check",
					Description: "Check service health",
					Tags:        []string{"health"},
					OperationID: "healthCheck",
				},
			},
		},
	}

	resources, templates := server.generateResourcesAndTemplates()

	// Should have 2 regular resources (non-parameterized endpoints)
	if len(resources) != 2 {
		t.Errorf("Expected 2 resources, got %d", len(resources))
	}

	// Should have 1 resource template (parameterized endpoint)
	if len(templates) != 1 {
		t.Errorf("Expected 1 resource template, got %d", len(templates))
	}

	// Check that the parameterized endpoint became a template
	found := false
	for _, template := range templates {
		if template.URITemplate.Raw() == "quay://api/v1/repository/{namespace}/{repository}" {
			found = true
			if template.Name != "Get repository" {
				t.Errorf("Expected template name 'Get repository', got '%s'", template.Name)
			}
			break
		}
	}
	if !found {
		t.Error("Expected parameterized endpoint to be a resource template")
	}

	// Check that non-parameterized endpoints became resources
	resourceURIs := make(map[string]bool)
	for _, resource := range resources {
		resourceURIs[resource.URI] = true
	}

	expectedResourceURIs := []string{
		"quay://api/v1/user",
		"quay://api/v1/health",
	}

	for _, expectedURI := range expectedResourceURIs {
		if !resourceURIs[expectedURI] {
			t.Errorf("Expected resource URI '%s' not found", expectedURI)
		}
	}
}

func TestGenerateResourceTemplates(t *testing.T) {
	server := NewQuayMCPServer("https://quay.io", "")

	// Mock swagger spec
	server.quayClient.spec = &SwaggerSpec{
		Paths: map[string]PathDetails{
			"/api/v1/repository/{namespace}/{repository}": {
				Get: &OperationDetails{
					Summary:     "Get repository",
					Description: "Get information about a repository",
					Tags:        []string{"repository"},
					OperationID: "getRepository",
				},
			},
			"/api/v1/user/{username}": {
				Get: &OperationDetails{
					Summary:     "Get user",
					Description: "Get information about a user",
					Tags:        []string{"user"},
					OperationID: "getUser",
				},
			},
		},
	}

	_, templates := server.generateResourcesAndTemplates()

	if len(templates) != 2 {
		t.Errorf("Expected 2 resource templates, got %d", len(templates))
	}

	// Check first template
	found := false
	for _, template := range templates {
		if template.URITemplate.Raw() == "quay://api/v1/repository/{namespace}/{repository}" {
			found = true
			if template.Name != "Get repository" {
				t.Errorf("Expected template name 'Get repository', got '%s'", template.Name)
			}
			break
		}
	}
	if !found {
		t.Error("Expected repository template not found")
	}
}

func TestExtractPathParameters(t *testing.T) {
	client := NewQuayClient("https://quay.io", "")

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
		result := client.extractPathParameters(test.resourceURI, test.pathTemplate)

		if len(result) != len(test.expected) {
			t.Errorf("Expected %d parameters, got %d for URI %s", len(test.expected), len(result), test.resourceURI)
			continue
		}

		for key, expectedValue := range test.expected {
			if actualValue, exists := result[key]; !exists || actualValue != expectedValue {
				t.Errorf("Expected parameter %s=%s, got %s=%s for URI %s", key, expectedValue, key, actualValue, test.resourceURI)
			}
		}
	}
}

func TestBuildAPIURL(t *testing.T) {
	client := NewQuayClient("https://quay.io", "")
	client.spec = &SwaggerSpec{
		BasePath: "/api/v1",
	}

	endpoint := &EndpointInfo{
		Method: "GET",
		Path:   "/api/v1/repository/{namespace}/{repository}",
	}

	url, err := client.BuildAPIURL(endpoint, "quay://api/v1/repository/myorg/myrepo")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	expected := "https://quay.io/api/v1/api/v1/repository/myorg/myrepo"
	if url != expected {
		t.Errorf("Expected URL '%s', got '%s'", expected, url)
	}
}

func TestMakeAPICall(t *testing.T) {
	// Create a mock server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message": "success"}`))
	}))
	defer mockServer.Close()

	client := NewQuayClient(mockServer.URL, "")
	client.spec = &SwaggerSpec{}

	endpoint := &EndpointInfo{
		Method: "GET",
		Path:   "/test",
	}

	data, err := client.MakeAPICall(endpoint, "quay://test")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	expected := `{"message": "success"}`
	if string(data) != expected {
		t.Errorf("Expected response '%s', got '%s'", expected, string(data))
	}
}

func TestOAuthTokenInAPICall(t *testing.T) {
	// Create a mock server that checks for the Authorization header
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader != "Bearer test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error": "unauthorized"}`))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message": "authenticated"}`))
	}))
	defer mockServer.Close()

	client := NewQuayClient(mockServer.URL, "test-token")
	client.spec = &SwaggerSpec{}

	endpoint := &EndpointInfo{
		Method: "GET",
		Path:   "/test",
	}

	data, err := client.MakeAPICall(endpoint, "quay://test")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	expected := `{"message": "authenticated"}`
	if string(data) != expected {
		t.Errorf("Expected response '%s', got '%s'", expected, string(data))
	}
}
