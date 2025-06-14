package main

import (
	"encoding/json"
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

	// Generate resources and templates
	resources, templates := server.generateResourcesAndTemplates()

	fmt.Printf("Generated %d resources and %d resource templates from Quay API\n\n", len(resources), len(templates))

	// Show first few resources (non-parameterized endpoints)
	fmt.Println("Sample Resources (non-parameterized endpoints):")
	for i, resource := range resources {
		if i >= 3 { // Show only first 3
			break
		}
		fmt.Printf("- %s: %s\n", resource.URI, resource.Name)
	}

	// Show first few resource templates (parameterized endpoints)
	fmt.Println("\nSample Resource Templates (parameterized endpoints):")
	for i, template := range templates {
		if i >= 3 { // Show only first 3
			break
		}
		fmt.Printf("- %s: %s\n", template.URITemplate.Raw(), template.Name)
	}

	// Show some endpoint details
	fmt.Println("\nEndpoint details:")
	count := 0
	endpoints := server.quayClient.GetEndpoints()
	for uri, endpoint := range endpoints {
		if count >= 5 { // Show only first 5
			break
		}
		fmt.Printf("- %s -> %s %s (%s)\n", uri, endpoint.Method, endpoint.Path, endpoint.Summary)
		count++
	}

	// Show the swagger spec structure
	spec := server.quayClient.GetSpec()
	fmt.Printf("\nSwagger spec loaded from: %s/api/v1/discovery\n", server.quayClient.GetRegistryURL())
	fmt.Printf("Host: %s\n", spec.Host)
	fmt.Printf("Base Path: %s\n", spec.BasePath)
	fmt.Printf("Schemes: %v\n", spec.Schemes)

	// Pretty print a sample endpoint
	if len(spec.Paths) > 0 {
		fmt.Println("\nSample endpoint structure:")
		for path, details := range spec.Paths {
			if details.Get != nil {
				jsonData, _ := json.MarshalIndent(map[string]interface{}{
					"path":      path,
					"operation": details.Get,
				}, "", "  ")
				fmt.Println(string(jsonData))
				break // Show only one example
			}
		}
	}
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
