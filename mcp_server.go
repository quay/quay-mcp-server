package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// QuayMCPServer wraps the MCP server with Quay-specific functionality
type QuayMCPServer struct {
	quayClient *QuayClient
	mcpServer  *server.MCPServer
}

// NewQuayMCPServer creates a new Quay MCP server
func NewQuayMCPServer(registryURL, oauthToken string) *QuayMCPServer {
	return &QuayMCPServer{
		quayClient: NewQuayClient(registryURL, oauthToken),
		mcpServer: server.NewMCPServer(
			"quay-mcp",
			"1.0.0",
			server.WithResourceCapabilities(true, true), // Enable both resources and resource templates
		),
	}
}

// generateResourcesAndTemplates creates MCP resources and templates from Quay API endpoints
func (s *QuayMCPServer) generateResourcesAndTemplates() ([]mcp.Resource, []mcp.ResourceTemplate) {
	spec := s.quayClient.GetSpec()
	if spec == nil {
		return nil, nil
	}

	var resources []mcp.Resource
	var templates []mcp.ResourceTemplate

	for path, pathDetails := range spec.Paths {
		// Only process GET operations
		if pathDetails.Get == nil {
			continue
		}

		operation := pathDetails.Get

		// Create name and description
		name := operation.Summary
		if name == "" {
			name = operation.Description
		}
		if name == "" {
			name = fmt.Sprintf("GET %s", path)
		}

		description := fmt.Sprintf("GET %s", path)
		if operation.Description != "" {
			description += fmt.Sprintf(" - %s", operation.Description)
		}
		if len(operation.Tags) > 0 {
			description += fmt.Sprintf(" (Tags: %s)", strings.Join(operation.Tags, ", "))
		}
		if operation.OperationID != "" {
			description += fmt.Sprintf(" [%s]", operation.OperationID)
		}

		uri := fmt.Sprintf("quay://%s", strings.TrimPrefix(path, "/"))

		// Check if the path has parameters
		if HasPathParameters(path) {
			// Create a resource template for parameterized endpoints
			template := mcp.NewResourceTemplate(
				uri,
				name,
				mcp.WithTemplateDescription(description),
				mcp.WithTemplateMIMEType("application/json"),
			)
			templates = append(templates, template)
		} else {
			// Create a regular resource for non-parameterized endpoints
			resource := mcp.NewResource(
				uri,
				name,
				mcp.WithResourceDescription(description),
				mcp.WithMIMEType("application/json"),
			)
			resources = append(resources, resource)
		}
	}

	return resources, templates
}

// createResourceHandler creates a handler function for MCP resource requests
func (s *QuayMCPServer) createResourceHandler() func(context.Context, mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	return func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		// Get endpoint info for this resource
		endpoints := s.quayClient.GetEndpoints()
		endpoint, exists := endpoints[request.Params.URI]
		if !exists {
			return nil, fmt.Errorf("endpoint not found for URI: %s", request.Params.URI)
		}

		// Make the actual API call to Quay (all endpoints are GET requests)
		responseData, err := s.quayClient.MakeAPICall(endpoint, request.Params.URI)
		if err != nil {
			// Return error information as JSON
			errorInfo := map[string]interface{}{
				"uri":    request.Params.URI,
				"error":  err.Error(),
				"method": endpoint.Method,
				"path":   endpoint.Path,
			}

			jsonData, marshalErr := json.MarshalIndent(errorInfo, "", "  ")
			if marshalErr != nil {
				return nil, fmt.Errorf("API call failed: %v (and failed to marshal error: %v)", err, marshalErr)
			}

			return []mcp.ResourceContents{
				mcp.TextResourceContents{
					URI:      request.Params.URI,
					MIMEType: "application/json",
					Text:     string(jsonData),
				},
			}, nil
		}

		// Validate that response is valid JSON
		var jsonCheck interface{}
		if err := json.Unmarshal(responseData, &jsonCheck); err != nil {
			// If not valid JSON, wrap it
			wrappedResponse := map[string]interface{}{
				"uri":      request.Params.URI,
				"response": string(responseData),
				"note":     "Response was not valid JSON, wrapped as string",
			}

			jsonData, marshalErr := json.MarshalIndent(wrappedResponse, "", "  ")
			if marshalErr != nil {
				return nil, fmt.Errorf("failed to process response: %v", marshalErr)
			}
			responseData = jsonData
		}

		return []mcp.ResourceContents{
			mcp.TextResourceContents{
				URI:      request.Params.URI,
				MIMEType: "application/json",
				Text:     string(responseData),
			},
		}, nil
	}
}

// Start initializes and starts the MCP server
func (s *QuayMCPServer) Start() error {
	// Fetch swagger spec
	if err := s.quayClient.FetchSwaggerSpec(); err != nil {
		return fmt.Errorf("failed to fetch swagger spec: %v", err)
	}

	// Discover endpoints
	s.quayClient.DiscoverEndpoints()

	// Generate and add resources and templates
	resources, templates := s.generateResourcesAndTemplates()

	// Create a shared resource handler
	resourceHandler := s.createResourceHandler()

	// Add regular resources (for endpoints without parameters)
	for _, resource := range resources {
		// Capture the resource in the closure
		currentResource := resource
		s.mcpServer.AddResource(currentResource, resourceHandler)
	}

	// Add resource templates (for endpoints with parameters)
	for _, template := range templates {
		// Capture the template in the closure
		currentTemplate := template
		s.mcpServer.AddResourceTemplate(currentTemplate, resourceHandler)
	}

	// Start the server using stdio
	return server.ServeStdio(s.mcpServer)
}
