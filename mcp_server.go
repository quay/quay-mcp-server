package main

import (
	"context"
	"fmt"
	"log"
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
			server.WithToolCapabilities(false), // Enable tools
		),
	}
}

// createToolHandler creates a handler function for MCP tool calls
func (s *QuayMCPServer) createToolHandler() func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Extract tool name and find corresponding endpoint
		toolName := request.Params.Name

		// Remove "quay_" prefix to get the original identifier
		if !strings.HasPrefix(toolName, "quay_") {
			return mcp.NewToolResultError("Invalid tool name: must start with 'quay_'"), nil
		}

		identifier := strings.TrimPrefix(toolName, "quay_")

		// Find the endpoint by operation ID or path
		var endpoint *EndpointInfo

		endpoints := s.quayClient.GetEndpoints()
		arguments := request.GetArguments()

		// First try to find by operation ID
		for _, ep := range endpoints {
			if ep.OperationID == identifier {
				endpoint = ep
				break
			}
		}

		// If not found by operation ID, try to find by path-based identifier
		if endpoint == nil {
			for _, ep := range endpoints {
				pathIdentifier := strings.ReplaceAll(strings.Trim(ep.Path, "/"), "/", "_")
				pathIdentifier = strings.ReplaceAll(pathIdentifier, "{", "")
				pathIdentifier = strings.ReplaceAll(pathIdentifier, "}", "")
				if pathIdentifier == "" {
					pathIdentifier = "root"
				}

				if pathIdentifier == identifier {
					endpoint = ep
					break
				}
			}
		}

		if endpoint == nil {
			return mcp.NewToolResultError(fmt.Sprintf("Endpoint not found for tool: %s", toolName)), nil
		}

		// Use the new method that handles both path and query parameters for all endpoints
		log.Printf("Making API call to endpoint: %s %s", endpoint.Method, endpoint.Path)
		log.Printf("With arguments: %+v", arguments)

		// Handle custom resource_uri if provided - but only for path parameter construction
		if customURI, exists := arguments["resource_uri"]; exists {
			if customURIStr, ok := customURI.(string); ok && customURIStr != "" {
				// For custom resource URIs, we might need to use the old method
				// if it's a complete custom URI that doesn't follow our parameter pattern
				if HasPathParameters(endpoint.Path) {
					// Still use the new method but log the custom URI usage
					log.Printf("Custom resource_uri provided but endpoint has path parameters, using new method")
				}
			}
		}

		responseData, err := s.quayClient.MakeAPICallWithParams(endpoint, arguments)
		if err != nil {
			return mcp.NewToolResultText(fmt.Sprintf("API call failed: %s", err.Error())), nil
		}

		// Return the JSON response as text
		return mcp.NewToolResultText(string(responseData)), nil
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

	// Generate and add tools
	tools := s.quayClient.generateTools()

	// Create a shared tool handler
	toolHandler := s.createToolHandler()

	// Add all tools
	for _, tool := range tools {
		// Capture the tool in the closure
		currentTool := tool
		s.mcpServer.AddTool(currentTool, toolHandler)
	}

	// Start the server using stdio
	return server.ServeStdio(s.mcpServer)
}
