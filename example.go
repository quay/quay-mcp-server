package main

import (
	"fmt"
	"log"
)

// Example demonstrates how to use the Quay MCP Server
func ExampleUsage() {
	// Create a new Quay MCP server
	server := NewQuayMCPServer("https://quay.io", "")

	// Fetch the swagger spec
	if err := server.quayClient.FetchSwaggerSpec(); err != nil {
		log.Fatalf("Failed to fetch swagger spec: %v", err)
	}

	// Discover endpoints
	server.quayClient.DiscoverEndpoints()

	// Generate and display tools
	tools := server.quayClient.generateTools()

	fmt.Printf("\nFound %d tools from the API:\n", len(tools))

	for i, tool := range tools {
		if i >= 3 { // Show only first 3 for brevity
			fmt.Printf("... and %d more tools\n", len(tools)-i)
			break
		}

		fmt.Printf("- Tool: %s\n", tool.Name)
		if tool.Description != "" {
			fmt.Printf("  Description: %s\n", tool.Description)
		}

		// Show required parameters
		if len(tool.InputSchema.Required) > 0 {
			fmt.Printf("  Required parameters: %v\n", tool.InputSchema.Required)
		}
	}

	fmt.Println("\nTo use this as an MCP server, run:")
	fmt.Println("  ./quay-mcp")
	fmt.Println("\nThen configure your Claude Desktop client to use this server.")
}

// This example would output something like:
//
// Generated 45 resource templates from Quay API (GET endpoints only):
// - template: List repositories
// - template: Get repository details
// - template: Get user information
// - template: Get organization details
// - template: Search repositories
// ... and 40 more resource templates
//
// Endpoint information stored for 45 GET endpoints:
// - quay://repositories -> GET /repositories
// - quay://repository/{repository} -> GET /repository/{repository}
// - quay://user/{username} -> GET /user/{username}
// ... and 42 more endpoints
//
// Key Features:
// - Only GET endpoints are processed (safe, read-only operations)
// - Each resource template corresponds to a live API endpoint
// - Path parameters are automatically extracted and substituted
// - All resource template instantiations result in actual API calls to Quay
// - Responses contain live data from the Quay registry
// - Templates provide flexible, parameterized access to Quay API endpoints

// ExampleResourceTemplateInstantiation demonstrates how a client would use resource templates
func ExampleResourceTemplateInstantiation() {
	// Example of how an MCP client would instantiate resource templates:
	//
	// 1. Client discovers available resource templates from the server
	// 2. Client selects a template like "quay://repository/{repository}"
	// 3. Client instantiates the template with specific parameters:
	//    - URI: "quay://repository/myorg/myrepo"
	//    - This triggers an API call to: https://quay.io/api/v1/repository/myorg/myrepo
	// 4. Server returns live repository data from Quay
	//
	// Benefits of resource templates:
	// - Parameterized: One template can handle many similar resources
	// - Type-safe: Templates define the expected parameter structure
	// - Discoverable: Clients can enumerate available templates
	// - Flexible: Templates can be instantiated with different parameter values
}

// ExampleAPICall demonstrates how the server would handle an actual resource request
func ExampleAPICall() {
	fmt.Println("When a client requests a resource:")
	fmt.Println()

	fmt.Println("1. GET endpoint (returns live data):")
	fmt.Println("   Request: quay://repositories")
	fmt.Println("   Response: Live JSON data from Quay API")
	fmt.Println()

	fmt.Println("2. POST endpoint (returns metadata):")
	fmt.Println("   Request: quay://repository/myorg/myrepo")
	fmt.Println("   Response: Endpoint metadata (no API call made)")
	fmt.Println()

	fmt.Println("3. API error (returns error info):")
	fmt.Println("   Request: quay://repository/nonexistent/repo")
	fmt.Println("   Response: Error details in JSON format")
}

// RunExample demonstrates the Quay MCP server functionality
func RunExample(registryURL, oauthToken string) {
	fmt.Printf("Connecting to Quay registry at: %s\n", registryURL)

	// Create Quay client
	client := NewQuayClient(registryURL, oauthToken)

	// Fetch and parse swagger spec
	if err := client.FetchSwaggerSpec(); err != nil {
		log.Fatalf("Failed to fetch swagger spec: %v", err)
	}

	// Discover endpoints
	client.DiscoverEndpoints()

	// Generate tools
	tools := client.generateTools()

	fmt.Printf("\nFound %d tools from the API:\n", len(tools))

	// Show first 3 tools in detail
	for i, tool := range tools {
		if i >= 3 {
			break
		}

		fmt.Printf("- Tool: %s\n", tool.Name)
		fmt.Printf("  Description: %s\n", tool.Description)

		// Show any required parameters
		if tool.InputSchema.Required != nil && len(tool.InputSchema.Required) > 0 {
			// Filter out the "resource_uri" parameter since it's optional
			var requiredParams []string
			for _, param := range tool.InputSchema.Required {
				if param != "resource_uri" {
					requiredParams = append(requiredParams, param)
				}
			}
			if len(requiredParams) > 0 {
				fmt.Printf("  Required parameters: %v\n", requiredParams)
			}
		}
	}

	if len(tools) > 3 {
		fmt.Printf("... and %d more tools\n", len(tools)-3)
	}

	// Try to make a sample API call to demonstrate logging
	fmt.Printf("\n=== MAKING SAMPLE API CALL ===\n")
	endpoints := client.GetEndpoints()

	// Find a simple endpoint like /api/v1/plans/
	var sampleEndpoint *EndpointInfo
	var sampleURI string

	for uri, endpoint := range endpoints {
		if endpoint.Path == "/api/v1/plans/" && endpoint.Method == "GET" {
			sampleEndpoint = endpoint
			sampleURI = uri
			break
		}
	}

	if sampleEndpoint != nil {
		fmt.Printf("Making sample API call to demonstrate logging...\n")
		data, err := client.MakeAPICall(sampleEndpoint, sampleURI)
		if err != nil {
			fmt.Printf("Sample API call failed: %v\n", err)
		} else {
			fmt.Printf("Sample API call succeeded, received %d bytes of data\n", len(data))
		}
	} else {
		fmt.Printf("Could not find sample endpoint for demonstration\n")
	}

	// Display information about the swagger spec
	model := client.GetModel()
	if model != nil {
		fmt.Printf("\nSwagger spec loaded from: %s\n", registryURL+"/api/v1/discovery")
		fmt.Printf("Host: %s\n", model.Model.Host)
		fmt.Printf("Base Path: %s\n", model.Model.BasePath)
		fmt.Printf("Schemes: %v\n", model.Model.Schemes)
		if model.Model.Info != nil {
			fmt.Printf("Title: %s\n", model.Model.Info.Title)
			fmt.Printf("Version: %s\n", model.Model.Info.Version)
		}

		// Show structure of a sample endpoint
		for pathPair := model.Model.Paths.PathItems.First(); pathPair != nil; pathPair = pathPair.Next() {
			path := pathPair.Key()
			pathItem := pathPair.Value()

			if pathItem.Get != nil {
				fmt.Printf("\nSample endpoint structure:\n")
				fmt.Printf("{\n")
				fmt.Printf("  \"operation\": {\n")
				fmt.Printf("    \"description\": \"%s\",\n", pathItem.Get.Description)
				fmt.Printf("    \"operationId\": \"%s\",\n", pathItem.Get.OperationId)
				fmt.Printf("    \"summary\": \"%s\",\n", pathItem.Get.Summary)
				fmt.Printf("    \"tags\": %v\n", pathItem.Get.Tags)
				fmt.Printf("  },\n")
				fmt.Printf("  \"path\": \"%s\"\n", path)
				fmt.Printf("}\n")
				break
			}
		}
	}

	fmt.Printf("\nTo use this as an MCP server, run:\n")
	fmt.Printf("  ./quay-mcp -url %s\n", registryURL)
	fmt.Printf("\nThen configure your Claude Desktop client to use this server.\n")
}
