package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
)

// SwaggerSpec represents the structure of a Swagger/OpenAPI specification
type SwaggerSpec struct {
	Host     string                 `json:"host"`
	BasePath string                 `json:"basePath"`
	Schemes  []string               `json:"schemes"`
	Paths    map[string]PathDetails `json:"paths"`
}

// PathDetails represents the details of a specific API path
type PathDetails struct {
	XName      string                   `json:"x-name,omitempty"`
	XPath      string                   `json:"x-path,omitempty"`
	XTag       string                   `json:"x-tag,omitempty"`
	Parameters []map[string]interface{} `json:"parameters,omitempty"`
	Get        *OperationDetails        `json:"get,omitempty"`
	Post       *OperationDetails        `json:"post,omitempty"`
	Put        *OperationDetails        `json:"put,omitempty"`
	Delete     *OperationDetails        `json:"delete,omitempty"`
	Patch      *OperationDetails        `json:"patch,omitempty"`
	Head       *OperationDetails        `json:"head,omitempty"`
	Options    *OperationDetails        `json:"options,omitempty"`
}

// OperationDetails represents the details of a specific HTTP operation
type OperationDetails struct {
	Summary     string                   `json:"summary,omitempty"`
	Description string                   `json:"description,omitempty"`
	Parameters  []map[string]interface{} `json:"parameters,omitempty"`
	Responses   map[string]interface{}   `json:"responses,omitempty"`
	Tags        []string                 `json:"tags,omitempty"`
	OperationID string                   `json:"operationId,omitempty"`
	Security    []map[string]interface{} `json:"security,omitempty"`
}

// EndpointInfo stores information about a discovered API endpoint
type EndpointInfo struct {
	Method      string
	Path        string
	Summary     string
	OperationID string
	Tags        []string
	Parameters  []map[string]interface{}
}

// QuayClient handles all interactions with the Quay registry API
type QuayClient struct {
	registryURL string
	oauthToken  string
	spec        *SwaggerSpec
	endpoints   map[string]*EndpointInfo // URI -> EndpointInfo mapping
}

// NewQuayClient creates a new Quay client for the given registry URL and optional OAuth token
func NewQuayClient(registryURL, oauthToken string) *QuayClient {
	return &QuayClient{
		registryURL: strings.TrimRight(registryURL, "/"),
		oauthToken:  oauthToken,
		endpoints:   make(map[string]*EndpointInfo),
	}
}

// FetchSwaggerSpec fetches and parses the Swagger specification from the Quay registry
func (c *QuayClient) FetchSwaggerSpec() error {
	// Construct the discovery URL - try /api/v1/discovery first, then fall back to /discovery
	discoveryURL := strings.TrimSuffix(c.registryURL, "/") + "/api/v1/discovery"

	resp, err := http.Get(discoveryURL)
	if err != nil {
		return fmt.Errorf("failed to fetch swagger spec: %w", err)
	}
	defer resp.Body.Close()

	// If /api/v1/discovery fails with 404, try /discovery as fallback
	if resp.StatusCode == 404 {
		discoveryURL = strings.TrimSuffix(c.registryURL, "/") + "/discovery"
		resp, err = http.Get(discoveryURL)
		if err != nil {
			return fmt.Errorf("failed to fetch swagger spec from fallback URL: %w", err)
		}
		defer resp.Body.Close()
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to fetch swagger spec: status code %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read swagger spec: %w", err)
	}

	if err := json.Unmarshal(body, &c.spec); err != nil {
		return fmt.Errorf("failed to parse swagger spec: %w", err)
	}

	log.Printf("Successfully loaded Quay API with %d endpoints", len(c.spec.Paths))
	return nil
}

// GetRegistryURL returns the registry URL
func (c *QuayClient) GetRegistryURL() string {
	return c.registryURL
}

// GetSpec returns the loaded Swagger specification
func (c *QuayClient) GetSpec() *SwaggerSpec {
	return c.spec
}

// GetEndpoints returns the discovered endpoints
func (c *QuayClient) GetEndpoints() map[string]*EndpointInfo {
	return c.endpoints
}

// DiscoverEndpoints processes the Swagger spec and discovers all GET endpoints
func (c *QuayClient) DiscoverEndpoints() {
	if c.spec == nil {
		return
	}

	for path, pathDetails := range c.spec.Paths {
		// Only process GET operations
		if pathDetails.Get == nil {
			continue
		}

		operation := pathDetails.Get
		uri := fmt.Sprintf("quay://%s", strings.TrimPrefix(path, "/"))

		// Store endpoint info for later API calls
		c.endpoints[uri] = &EndpointInfo{
			Method:      "GET",
			Path:        path,
			Summary:     operation.Summary,
			OperationID: operation.OperationID,
			Tags:        operation.Tags,
			Parameters:  operation.Parameters,
		}
	}
}

// HasPathParameters checks if a path contains parameters (e.g., {id})
func HasPathParameters(path string) bool {
	return strings.Contains(path, "{") && strings.Contains(path, "}")
}

// BuildAPIURL constructs the full API URL for a given endpoint and resource URI
func (c *QuayClient) BuildAPIURL(endpoint *EndpointInfo, resourceURI string) (string, error) {
	// Start with the registry URL
	baseURL := c.registryURL

	// Add base path from swagger if available
	if c.spec.BasePath != "" {
		baseURL = strings.TrimRight(baseURL, "/") + "/" + strings.TrimLeft(c.spec.BasePath, "/")
	}

	// Add the endpoint path
	fullURL := strings.TrimRight(baseURL, "/") + endpoint.Path

	// Extract any path parameters from the resource URI
	pathParams := c.extractPathParameters(resourceURI, endpoint.Path)
	for param, value := range pathParams {
		placeholder := fmt.Sprintf("{%s}", param)
		fullURL = strings.ReplaceAll(fullURL, placeholder, value)
	}

	return fullURL, nil
}

// extractPathParameters extracts path parameters from a resource URI based on a path template
func (c *QuayClient) extractPathParameters(resourceURI, pathTemplate string) map[string]string {
	params := make(map[string]string)

	// Remove the "quay://" prefix from resourceURI
	resourcePath := strings.TrimPrefix(resourceURI, "quay://")
	if !strings.HasPrefix(resourcePath, "/") {
		resourcePath = "/" + resourcePath
	}

	// Convert path template to regex pattern
	// Replace {param} with named capture groups
	regexPattern := pathTemplate
	paramNames := []string{}

	// Find all {param} patterns
	paramRegex := regexp.MustCompile(`\{([^}]+)\}`)
	matches := paramRegex.FindAllStringSubmatch(pathTemplate, -1)

	for _, match := range matches {
		paramName := match[1]
		paramNames = append(paramNames, paramName)
		// Replace {param} with ([^/]+) capture group
		regexPattern = strings.ReplaceAll(regexPattern, match[0], "([^/]+)")
	}

	// Compile and match against the resource path
	if len(paramNames) > 0 {
		regex, err := regexp.Compile("^" + regexPattern + "$")
		if err == nil {
			matches := regex.FindStringSubmatch(resourcePath)
			if len(matches) > 1 {
				for i, paramName := range paramNames {
					if i+1 < len(matches) {
						params[paramName] = matches[i+1]
					}
				}
			}
		}
	}

	return params
}

// MakeAPICall makes an HTTP request to the Quay API and returns the response
func (c *QuayClient) MakeAPICall(endpoint *EndpointInfo, resourceURI string) ([]byte, error) {
	apiURL, err := c.BuildAPIURL(endpoint, resourceURI)
	if err != nil {
		return nil, fmt.Errorf("failed to build API URL: %v", err)
	}

	// Create HTTP request
	req, err := http.NewRequest(endpoint.Method, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %v", err)
	}

	// Set headers
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "quay-mcp-server/1.0.0")

	// Add OAuth token if provided
	if c.oauthToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.oauthToken)
	}

	// Make the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make API request: %v", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	// Check for HTTP errors
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	return body, nil
}
