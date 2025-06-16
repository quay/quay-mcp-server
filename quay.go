package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/pb33f/libopenapi"
	v2high "github.com/pb33f/libopenapi/datamodel/high/v2"
)

// EndpointInfo stores information about a discovered API endpoint
type EndpointInfo struct {
	Method      string
	Path        string
	Summary     string
	OperationID string
	Tags        []string
	Parameters  []interface{}
}

// QuayClient handles all interactions with the Quay registry API
type QuayClient struct {
	registryURL string
	oauthToken  string
	document    libopenapi.Document
	model       *libopenapi.DocumentModel[v2high.Swagger]
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

	log.Printf("=== FETCHING SWAGGER SPEC ===")
	log.Printf("Registry URL: %s", c.registryURL)
	log.Printf("Discovery URL: %s", discoveryURL)

	resp, err := http.Get(discoveryURL)
	if err != nil {
		log.Printf("Failed to fetch from primary discovery URL: %v", err)
		return fmt.Errorf("failed to fetch swagger spec: %w", err)
	}
	defer resp.Body.Close()

	// If /api/v1/discovery fails with 404, try /discovery as fallback
	if resp.StatusCode == 404 {
		log.Printf("Primary discovery URL returned 404, trying fallback...")
		discoveryURL = strings.TrimSuffix(c.registryURL, "/") + "/discovery"
		log.Printf("Fallback URL: %s", discoveryURL)

		resp, err = http.Get(discoveryURL)
		if err != nil {
			log.Printf("Failed to fetch from fallback discovery URL: %v", err)
			return fmt.Errorf("failed to fetch swagger spec from fallback URL: %w", err)
		}
		defer resp.Body.Close()
	}

	log.Printf("Discovery response status: %d %s", resp.StatusCode, resp.Status)
	log.Printf("Discovery response headers:")
	for name, values := range resp.Header {
		for _, value := range values {
			log.Printf("  %s: %s", name, value)
		}
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("Discovery request failed with status: %d", resp.StatusCode)
		return fmt.Errorf("failed to fetch swagger spec: status code %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Failed to read discovery response body: %v", err)
		return fmt.Errorf("failed to read swagger spec: %w", err)
	}

	log.Printf("Discovery response body size: %d bytes", len(body))

	// Log a sample of the spec for debugging (first 500 chars)
	bodyStr := string(body)
	if len(bodyStr) > 500 {
		log.Printf("Swagger spec preview: %s...", bodyStr[:500])
	} else {
		log.Printf("Swagger spec content: %s", bodyStr)
	}

	// Create a new document from the specification bytes
	document, err := libopenapi.NewDocument(body)
	if err != nil {
		log.Printf("Failed to create swagger document: %v", err)
		return fmt.Errorf("failed to create swagger document: %w", err)
	}

	c.document = document
	log.Printf("Successfully created libopenapi document")

	// Build the V2 model from the document (Swagger 2.0)
	docModel, errors := document.BuildV2Model()
	if len(errors) > 0 {
		log.Printf("Warning: errors occurred while building Swagger model:")
		for _, buildErr := range errors {
			log.Printf("  - %v", buildErr)
		}
	}

	if docModel == nil {
		log.Printf("Failed to build Swagger v2 model - docModel is nil")
		return fmt.Errorf("failed to build Swagger v2 model")
	}

	c.model = docModel

	// Log some basic info about the loaded spec
	if c.model.Model.Info != nil {
		log.Printf("Loaded Swagger spec - Title: %s, Version: %s", c.model.Model.Info.Title, c.model.Model.Info.Version)
	}
	log.Printf("Swagger spec host: %s", c.model.Model.Host)
	log.Printf("Swagger spec base path: %s", c.model.Model.BasePath)
	log.Printf("Swagger spec schemes: %v", c.model.Model.Schemes)

	// Count the number of paths
	pathCount := 0
	if c.model.Model.Paths != nil {
		for pathPair := c.model.Model.Paths.PathItems.First(); pathPair != nil; pathPair = pathPair.Next() {
			pathCount++
		}
	}
	log.Printf("Discovered %d API paths", pathCount)
	log.Printf("==============================")

	log.Printf("Successfully loaded Quay API with Swagger specification")
	return nil
}

// GetRegistryURL returns the registry URL
func (c *QuayClient) GetRegistryURL() string {
	return c.registryURL
}

// GetDocument returns the loaded Swagger document
func (c *QuayClient) GetDocument() libopenapi.Document {
	return c.document
}

// GetModel returns the loaded Swagger model
func (c *QuayClient) GetModel() *libopenapi.DocumentModel[v2high.Swagger] {
	return c.model
}

// GetEndpoints returns the discovered endpoints
func (c *QuayClient) GetEndpoints() map[string]*EndpointInfo {
	return c.endpoints
}

// DiscoverEndpoints processes the Swagger spec and discovers all GET endpoints
func (c *QuayClient) DiscoverEndpoints() {
	if c.model == nil {
		return
	}

	// Define allowed tags
	allowedTags := map[string]bool{
		"manifest":     true,
		"organization": true,
		"repository":   true,
		"robot":        true,
		"tag":          true,
		"team": 				true,
	}

	log.Printf("Filtering endpoints to include only tags: %v", []string{"manifest", "organization", "repository", "robot", "tag"})

	totalEndpoints := 0
	filteredEndpoints := 0

	// Iterate through all paths using the ordered map API
	for pathPair := c.model.Model.Paths.PathItems.First(); pathPair != nil; pathPair = pathPair.Next() {
		path := pathPair.Key()
		pathItem := pathPair.Value()

		// Only process GET operations
		if pathItem.Get == nil {
			continue
		}

		totalEndpoints++
		operation := pathItem.Get

		// Check if the operation has any of the allowed tags
		hasAllowedTag := false
		if operation.Tags != nil {
			for _, tag := range operation.Tags {
				if allowedTags[tag] {
					hasAllowedTag = true
					break
				}
			}
		}

		// Skip if no allowed tags found
		if !hasAllowedTag {
			continue
		}

		filteredEndpoints++
		uri := fmt.Sprintf("quay://%s", strings.TrimPrefix(path, "/"))

		// Convert parameters to []interface{}
		var parameters []interface{}
		if operation.Parameters != nil {
			for _, param := range operation.Parameters {
				if param != nil {
					parameters = append(parameters, param)
				}
			}
		}

		// Store endpoint info for later API calls
		c.endpoints[uri] = &EndpointInfo{
			Method:      "GET",
			Path:        path,
			Summary:     operation.Summary,
			OperationID: operation.OperationId,
			Tags:        operation.Tags,
			Parameters:  parameters,
		}
	}

	log.Printf("Filtered %d/%d GET endpoints based on allowed tags", filteredEndpoints, totalEndpoints)
}

// HasPathParameters checks if a path contains parameters (e.g., {id})
func HasPathParameters(path string) bool {
	return strings.Contains(path, "{") && strings.Contains(path, "}")
}

// BuildAPIURL constructs the full API URL for a given endpoint and resource URI
func (c *QuayClient) BuildAPIURL(endpoint *EndpointInfo, resourceURI string) (string, error) {
	// Start with the registry URL
	baseURL := c.registryURL

	// Add base path from Swagger spec if available
	if c.model != nil && c.model.Model.BasePath != "" {
		baseURL = strings.TrimRight(baseURL, "/") + "/" + strings.TrimLeft(c.model.Model.BasePath, "/")
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

// BuildAPIURLWithParams constructs the full API URL for a given endpoint with explicit parameters
func (c *QuayClient) BuildAPIURLWithParams(endpoint *EndpointInfo, params map[string]interface{}) (string, error) {
	// Start with the registry URL
	baseURL := c.registryURL

	// Add base path from Swagger spec if available
	if c.model != nil && c.model.Model.BasePath != "" {
		baseURL = strings.TrimRight(baseURL, "/") + "/" + strings.TrimLeft(c.model.Model.BasePath, "/")
	}

	// Start with the endpoint path
	finalPath := endpoint.Path

	// Separate path parameters from query parameters
	pathParams := make(map[string]interface{})
	queryParams := make(map[string]interface{})

	// Get path parameter names
	pathParamNames := extractPathParameterNames(finalPath)
	pathParamMap := make(map[string]bool)
	for _, name := range pathParamNames {
		pathParamMap[name] = true
	}

	// Separate path and query parameters
	for key, value := range params {
		if key == "resource_uri" {
			continue // Skip the special resource_uri parameter
		}
		if pathParamMap[key] {
			pathParams[key] = value
		} else {
			// Assume it's a query parameter
			queryParams[key] = value
		}
	}

	// Replace path parameters with actual values
	if HasPathParameters(finalPath) {
		for _, paramName := range pathParamNames {
			if paramValue, exists := pathParams[paramName]; exists {
				if paramValueStr, ok := paramValue.(string); ok {
					placeholder := fmt.Sprintf("{%s}", paramName)
					finalPath = strings.ReplaceAll(finalPath, placeholder, paramValueStr)
				}
			}
		}
	}

	// Build the base URL
	fullURL := strings.TrimRight(baseURL, "/") + finalPath

	// Add query parameters if any
	if len(queryParams) > 0 {
		queryParts := []string{}
		for key, value := range queryParams {
			if valueStr, ok := value.(string); ok && valueStr != "" {
				queryParts = append(queryParts, fmt.Sprintf("%s=%s", key, url.QueryEscape(valueStr)))
			}
		}
		if len(queryParts) > 0 {
			fullURL += "?" + strings.Join(queryParts, "&")
		}
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

	// Log the outgoing request
	log.Printf("=== QUAY API REQUEST ===")
	log.Printf("Method: %s", req.Method)
	log.Printf("URL: %s", req.URL.String())
	log.Printf("Headers:")
	for name, values := range req.Header {
		for _, value := range values {
			// Mask the Authorization header for security
			if name == "Authorization" && strings.HasPrefix(value, "Bearer ") {
				log.Printf("  %s: Bearer [REDACTED]", name)
			} else {
				log.Printf("  %s: %s", name, value)
			}
		}
	}
	log.Printf("Resource URI: %s", resourceURI)
	log.Printf("Endpoint: %s %s (Operation: %s)", endpoint.Method, endpoint.Path, endpoint.OperationID)

	// Make the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("=== QUAY API REQUEST FAILED ===")
		log.Printf("Error: %v", err)
		return nil, fmt.Errorf("failed to make API request: %v", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("=== QUAY API RESPONSE READ FAILED ===")
		log.Printf("Error reading body: %v", err)
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	// Log the response
	log.Printf("=== QUAY API RESPONSE ===")
	log.Printf("Status: %d %s", resp.StatusCode, resp.Status)
	log.Printf("Headers:")
	for name, values := range resp.Header {
		for _, value := range values {
			log.Printf("  %s: %s", name, value)
		}
	}

	// Log response body (truncate if very long)
	bodyStr := string(body)
	if len(bodyStr) > 1000 {
		log.Printf("Response Body (%d bytes, truncated to 1000): %s...", len(bodyStr), bodyStr[:1000])
	} else {
		log.Printf("Response Body (%d bytes): %s", len(bodyStr), bodyStr)
	}
	log.Printf("========================")

	// Check for HTTP errors
	if resp.StatusCode >= 400 {
		log.Printf("API request failed with status %d", resp.StatusCode)
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	log.Printf("API request completed successfully")
	return body, nil
}

// MakeAPICallWithParams makes an HTTP request to the Quay API with explicit parameters and returns the response
func (c *QuayClient) MakeAPICallWithParams(endpoint *EndpointInfo, params map[string]interface{}) ([]byte, error) {
	apiURL, err := c.BuildAPIURLWithParams(endpoint, params)
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

	// Log the outgoing request
	log.Printf("=== QUAY API REQUEST ===")
	log.Printf("Method: %s", req.Method)
	log.Printf("URL: %s", req.URL.String())
	log.Printf("Headers:")
	for name, values := range req.Header {
		for _, value := range values {
			// Mask the Authorization header for security
			if name == "Authorization" && strings.HasPrefix(value, "Bearer ") {
				log.Printf("  %s: Bearer [REDACTED]", name)
			} else {
				log.Printf("  %s: %s", name, value)
			}
		}
	}
	log.Printf("Parameters: %v", params)
	log.Printf("Endpoint: %s %s (Operation: %s)", endpoint.Method, endpoint.Path, endpoint.OperationID)

	// Make the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("=== QUAY API REQUEST FAILED ===")
		log.Printf("Error: %v", err)
		return nil, fmt.Errorf("failed to make API request: %v", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("=== QUAY API RESPONSE READ FAILED ===")
		log.Printf("Error reading body: %v", err)
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	// Log the response
	log.Printf("=== QUAY API RESPONSE ===")
	log.Printf("Status: %d %s", resp.StatusCode, resp.Status)
	log.Printf("Headers:")
	for name, values := range resp.Header {
		for _, value := range values {
			log.Printf("  %s: %s", name, value)
		}
	}

	// Log response body (truncate if very long)
	bodyStr := string(body)
	if len(bodyStr) > 1000 {
		log.Printf("Response Body (%d bytes, truncated to 1000): %s...", len(bodyStr), bodyStr[:1000])
	} else {
		log.Printf("Response Body (%d bytes): %s", len(bodyStr), bodyStr)
	}
	log.Printf("========================")

	// Check for HTTP errors
	if resp.StatusCode >= 400 {
		log.Printf("API request failed with status %d", resp.StatusCode)
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	log.Printf("API request completed successfully")
	return body, nil
}

// generateTools creates MCP tools from Quay API endpoints
func (c *QuayClient) generateTools() []mcp.Tool {
	model := c.GetModel()
	if model == nil {
		return nil
	}

	// Define allowed tags
	allowedTags := map[string]bool{
		"manifest":     true,
		"organization": true,
		"repository":   true,
		"robot":        true,
		"tag":          true,
	}

	var tools []mcp.Tool

	// Iterate through all paths using the ordered map API
	for pathPair := model.Model.Paths.PathItems.First(); pathPair != nil; pathPair = pathPair.Next() {
		path := pathPair.Key()
		pathItem := pathPair.Value()

		// Only process GET operations
		if pathItem.Get == nil {
			continue
		}

		operation := pathItem.Get

		// Check if the operation has any of the allowed tags
		hasAllowedTag := false
		if operation.Tags != nil {
			for _, tag := range operation.Tags {
				if allowedTags[tag] {
					hasAllowedTag = true
					break
				}
			}
		}

		// Skip if no allowed tags found
		if !hasAllowedTag {
			continue
		}

		// Create tool name from operation ID or path
		toolName := operation.OperationId
		if toolName == "" {
			// Create a clean tool name from the path
			toolName = strings.ReplaceAll(strings.Trim(path, "/"), "/", "_")
			toolName = strings.ReplaceAll(toolName, "{", "")
			toolName = strings.ReplaceAll(toolName, "}", "")
			if toolName == "" {
				toolName = "root"
			}
		}
		toolName = "quay_" + toolName

		// Create description
		description := operation.Summary
		if description == "" {
			description = operation.Description
		}
		if description == "" {
			description = fmt.Sprintf("GET %s", path)
		}

		// Add additional context to description
		fullDescription := fmt.Sprintf("%s\nEndpoint: GET %s", description, path)
		if len(operation.Tags) > 0 {
			fullDescription += fmt.Sprintf("\nTags: %s", strings.Join(operation.Tags, ", "))
		}

		// Create tool options
		toolOptions := []mcp.ToolOption{
			mcp.WithDescription(fullDescription),
		}

		// Add path parameters to input schema
		if HasPathParameters(path) {
			// Extract parameter names from path
			pathParams := extractPathParameterNames(path)
			for _, paramName := range pathParams {
				toolOptions = append(toolOptions,
					mcp.WithString(paramName,
						mcp.Required(),
						mcp.Description(fmt.Sprintf("Path parameter: %s", paramName)),
					),
				)
			}
		}

		// Add query parameters from the operation
		if operation.Parameters != nil {
			for _, param := range operation.Parameters {
				if param != nil && param.In == "query" {
					paramName := param.Name
					paramDescription := param.Description
					if paramDescription == "" {
						paramDescription = fmt.Sprintf("Query parameter: %s", paramName)
					}

					// Query parameters are optional by default
					toolOptions = append(toolOptions,
						mcp.WithString(paramName,
							mcp.Description(paramDescription),
						),
					)
				}
			}
		}

		// Add a special "resource_uri" parameter for all tools to maintain compatibility
		toolOptions = append(toolOptions,
			mcp.WithString("resource_uri",
				mcp.Description("Optional: Custom resource URI (e.g., quay://repository/myorg/myrepo). If not provided, will be constructed from path parameters."),
			),
		)

		// Create the tool
		tool := mcp.NewTool(toolName, toolOptions...)

		tools = append(tools, tool)
	}

	return tools
}

// extractPathParameterNames extracts parameter names from a path template
func extractPathParameterNames(path string) []string {
	var paramNames []string

	// Find all {param} patterns
	start := 0
	for {
		startIdx := strings.Index(path[start:], "{")
		if startIdx == -1 {
			break
		}
		startIdx += start

		endIdx := strings.Index(path[startIdx:], "}")
		if endIdx == -1 {
			break
		}
		endIdx += startIdx

		paramName := path[startIdx+1 : endIdx]
		paramNames = append(paramNames, paramName)
		start = endIdx + 1
	}

	return paramNames
}
