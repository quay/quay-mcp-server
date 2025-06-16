package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
)

func runExample(server *QuayMCPServer) {
	fmt.Printf("Connecting to Quay registry at: %s\n", server.quayClient.GetRegistryURL())

	// Fetch and parse the OpenAPI specification
	err := server.quayClient.FetchSwaggerSpec()
	if err != nil {
		log.Fatalf("Failed to fetch swagger spec: %v", err)
	}

	// Discover endpoints from the parsed spec
	server.quayClient.DiscoverEndpoints()

	// Generate and display tools
	tools := server.quayClient.generateTools()

	fmt.Printf("\nFound %d tools from the API:\n", len(tools))

	// Show first 3 tools in detail
	for i, tool := range tools {
		if i >= 3 {
			break
		}

		fmt.Printf("- Tool: %s\n", tool.Name)
		fmt.Printf("  Description: %s\n", tool.Description)

		// Show any required parameters
		if len(tool.InputSchema.Required) > 0 {
			fmt.Printf("  Required parameters: %v\n", tool.InputSchema.Required)
		}

		// Show optional parameters (query parameters)
		if tool.InputSchema.Properties != nil {
			var optionalParams []string
			for paramName := range tool.InputSchema.Properties {
				isRequired := false
				for _, reqParam := range tool.InputSchema.Required {
					if reqParam == paramName {
						isRequired = true
						break
					}
				}
				if !isRequired && paramName != "resource_uri" {
					optionalParams = append(optionalParams, paramName)
				}
			}
			if len(optionalParams) > 0 {
				fmt.Printf("  Optional parameters: %v\n", optionalParams)
			}
		}
	}

	if len(tools) > 3 {
		fmt.Printf("... and %d more tools\n", len(tools)-3)
	}

	// Show the Swagger spec structure
	model := server.quayClient.GetModel()
	fmt.Printf("\nSwagger spec loaded from: %s/api/v1/discovery\n", server.quayClient.GetRegistryURL())

	if model != nil {
		// Access Swagger v2 information
		fmt.Printf("Host: %s\n", model.Model.Host)
		fmt.Printf("Base Path: %s\n", model.Model.BasePath)
		fmt.Printf("Schemes: %v\n", model.Model.Schemes)

		// Access info section
		if model.Model.Info != nil {
			fmt.Printf("Title: %s\n", model.Model.Info.Title)
			fmt.Printf("Version: %s\n", model.Model.Info.Version)
		}
	}

	// Try to make a sample API call to demonstrate logging
	fmt.Printf("\n=== MAKING SAMPLE API CALL ===\n")
	endpoints := server.quayClient.GetEndpoints()

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
		data, err := server.quayClient.MakeAPICall(sampleEndpoint, sampleURI)
		if err != nil {
			fmt.Printf("Sample API call failed: %v\n", err)
		} else {
			fmt.Printf("Sample API call succeeded, received %d bytes of data\n", len(data))
		}
	} else {
		fmt.Printf("Could not find sample endpoint for demonstration\n")
	}

	// Test parameter substitution
	fmt.Printf("\n=== TESTING PARAMETER SUBSTITUTION ===\n")

	// Find an endpoint with parameters
	var paramEndpoint *EndpointInfo
	for _, endpoint := range endpoints {
		// Look for the repository list endpoint which should have query parameters
		if endpoint.Path == "/api/v1/repository" {
			paramEndpoint = endpoint
			break
		}
	}

	// If we didn't find the repository endpoint, find any endpoint with parameters
	if paramEndpoint == nil {
		for _, endpoint := range endpoints {
			if HasPathParameters(endpoint.Path) {
				paramEndpoint = endpoint
				break
			}
		}
	}

	if paramEndpoint != nil {
		fmt.Printf("Testing with endpoint: %s %s\n", paramEndpoint.Method, paramEndpoint.Path)

		// Create test parameters including query parameters
		testParams := map[string]interface{}{
			"orgname":    "redhat",
			"repository": "quay/clair",
			"teamname":   "admins",
			"username":   "testuser",
			"namespace":  "redhat", // Common query parameter for repository listing
			"public":     "true",   // Another common query parameter
		}

		// Build URL with parameters
		finalURL, err := server.quayClient.BuildAPIURLWithParams(paramEndpoint, testParams)
		if err != nil {
			fmt.Printf("Error building URL: %v\n", err)
		} else {
			fmt.Printf("Original path: %s\n", paramEndpoint.Path)
			fmt.Printf("Test parameters: %v\n", testParams)
			fmt.Printf("Final URL: %s\n", finalURL)

			if HasPathParameters(finalURL) {
				fmt.Printf("❌ Warning: Some path parameters may not have been substituted\n")
			} else {
				fmt.Printf("✅ Success: All path parameters substituted correctly\n")
			}

			// Check if query parameters were added
			if strings.Contains(finalURL, "?") {
				fmt.Printf("✅ Success: Query parameters added to URL\n")
			} else {
				fmt.Printf("ℹ️  Info: No query parameters were added (may be expected if endpoint has none)\n")
			}
		}

		// Test actual MCP tool call with query parameters
		fmt.Printf("\n=== TESTING ACTUAL MCP TOOL CALL ===\n")
		if paramEndpoint.Path == "/api/v1/repository" {
			fmt.Printf("Making actual API call to demonstrate query parameter passing...\n")

			// Create test arguments with query parameters
			testArgs := map[string]interface{}{
				"namespace": "redhat",
				"public":    "true",
			}

			responseData, err := server.quayClient.MakeAPICallWithParams(paramEndpoint, testArgs)
			if err != nil {
				fmt.Printf("API call failed: %v\n", err)
			} else {
				fmt.Printf("API call succeeded, received %d bytes of data\n", len(responseData))
				// Show first 200 chars of response to verify it's working
				responseStr := string(responseData)
				if len(responseStr) > 200 {
					fmt.Printf("Response preview: %s...\n", responseStr[:200])
				} else {
					fmt.Printf("Response: %s\n", responseStr)
				}
			}
		} else {
			fmt.Printf("Repository endpoint not found for actual test\n")
		}
	} else {
		fmt.Printf("No endpoints found for testing\n")
	}

	// Pretty print a sample endpoint
	if model != nil && model.Model.Paths != nil {
		fmt.Println("\nSample endpoint structure:")
		// Iterate using the ordered map API
		for pathPair := model.Model.Paths.PathItems.First(); pathPair != nil; pathPair = pathPair.Next() {
			path := pathPair.Key()
			pathItem := pathPair.Value()

			if pathItem.Get != nil {
				jsonData, _ := json.MarshalIndent(map[string]interface{}{
					"path": path,
					"operation": map[string]interface{}{
						"summary":     pathItem.Get.Summary,
						"description": pathItem.Get.Description,
						"operationId": pathItem.Get.OperationId,
						"tags":        pathItem.Get.Tags,
					},
				}, "", "  ")
				fmt.Println(string(jsonData))
				break // Show only one example
			}
		}
	}

	fmt.Println("\nTo use this as an MCP server, run:")
	fmt.Println("  ./quay-mcp -url https://quay.io")
	fmt.Println("\nThen configure your Claude Desktop client to use this server.")
}

func main() {
	var registryURL string
	var oauthToken string
	var example bool

	// Define command-line flags
	flag.StringVar(&registryURL, "url", "", "Quay registry URL (required)")
	flag.StringVar(&oauthToken, "token", "", "OAuth token for authentication (optional)")
	flag.BoolVar(&example, "example", false, "Run example mode to show tools instead of starting server")

	// Custom usage message
	flag.Usage = func() {
		log.Printf("Usage: %s -url <quay-registry-url> [-token <oauth-token>] [-example]\n", os.Args[0])
		log.Println("\nFlags:")
		flag.PrintDefaults()
		log.Println("\nExamples:")
		log.Printf("  %s -url https://quay.io\n", os.Args[0])
		log.Printf("  %s -url https://quay.io -token your-oauth-token\n", os.Args[0])
		log.Printf("  %s -url https://quay.io -example\n", os.Args[0])
	}

	// Parse command-line flags
	flag.Parse()

	// Validate required arguments
	if registryURL == "" {
		log.Println("Error: registry URL is required")
		flag.Usage()
		os.Exit(1)
	}

	server := NewQuayMCPServer(registryURL, oauthToken)

	if example {
		// Run example mode
		runExample(server)
	} else {
		// Start the MCP server
		if err := server.Start(); err != nil {
			log.Fatalf("Failed to start server: %v", err)
		}
	}
}
