# Quay MCP Server

A Model Context Protocol (MCP) server that provides access to Quay container registry APIs. This server automatically discovers Quay API endpoints from the Swagger specification and exposes them as MCP resources and resource templates.

## Features

- **Automatic API Discovery**: Fetches Swagger spec from Quay registry and discovers all GET endpoints
- **Smart Resource Generation**: 
  - Non-parameterized endpoints become regular MCP resources
  - Parameterized endpoints become MCP resource templates
- **Live API Integration**: Makes real-time calls to Quay registry APIs
- **OAuth Authentication**: Supports Bearer token authentication for private registries
- **Clean Architecture**: Separated concerns between Quay API logic and MCP server logic

## Installation

```bash
go build -o quay-mcp
```

## Usage

```bash
# Basic usage (public endpoints only)
./quay-mcp -url <quay-registry-url>

# With OAuth token for authenticated access
./quay-mcp -url <quay-registry-url> -token <oauth-token>

# Examples
./quay-mcp -url https://quay.io
./quay-mcp -url https://quay.io -token your-oauth-token-here

# Get help
./quay-mcp -h
```

### Command-Line Flags

- `-url` (required): The Quay registry URL to connect to
- `-token` (optional): OAuth token for authentication
- `-h` or `-help`: Show usage information

### OAuth Token

For accessing private repositories or endpoints that require authentication, you can provide an OAuth token as the second argument. The token will be sent as a Bearer token in the Authorization header for all API requests.

To get an OAuth token for Quay.io:
1. Go to your Quay.io account settings
2. Navigate to "Robot Accounts" or "OAuth Applications"
3. Create a new token with appropriate permissions
4. Use the token with the server

## Architecture

The codebase is organized into three main files:

### `main.go` (Entry Point)
- Command-line argument parsing
- Server initialization and startup
- Minimal, focused on application bootstrap

### `quay.go` (Quay API Logic)
- `QuayClient` struct handling all Quay registry interactions
- Swagger spec fetching and parsing
- HTTP request handling with OAuth support
- API URL construction and parameter extraction
- Pure business logic, no MCP dependencies

### `mcp_server.go` (MCP Integration)
- `QuayMCPServer` struct wrapping QuayClient with MCP functionality
- Resource and resource template generation
- MCP request handling and response formatting
- Server lifecycle management

## How It Works

1. **Discovery Phase**: 
   - Fetches Swagger specification from `{registry-url}/api/v1/discovery`
   - Parses the spec to find all GET endpoints
   - Categorizes endpoints as resources or templates based on path parameters

2. **Resource Generation**:
   - **Resources**: Non-parameterized endpoints (e.g., `/api/v1/user/starred`)
   - **Resource Templates**: Parameterized endpoints (e.g., `/api/v1/repository/{repository}/manifest/{manifestref}`)

3. **Runtime**: 
   - MCP clients can list available resources and templates
   - When accessed, the server makes live API calls to the Quay registry
   - Responses are returned as JSON to the MCP client

## Example Output

From a real Quay.io instance:
- **Total GET endpoints**: 80
- **Regular resources**: 12 (non-parameterized)
- **Resource templates**: 46 (parameterized)

### Sample Resources
- `quay://api/v1/user/starred` - List starred repositories
- `quay://api/v1/repository` - Fetch visible repositories  
- `quay://api/v1/find/all` - Search entities

### Sample Resource Templates
- `quay://api/v1/repository/{repository}/manifest/{manifestref}/labels`
- `quay://api/v1/repository/{repository}/permissions/user/{username}`
- `quay://api/v1/organization/{orgname}/applications/{client_id}`

## Testing

```bash
go test -v
```

The test suite includes:
- Swagger spec fetching and parsing
- Resource and template generation logic
- API URL construction and parameter extraction
- HTTP request handling with OAuth token support
- Mock server testing for API interactions

## Development

To run the example and see the discovered endpoints:

```bash
go run example.go
```

This will show you all the resources and templates that would be generated from the Quay API.

## Dependencies

- `github.com/mark3labs/mcp-go` - MCP SDK for Go
- Standard Go libraries for HTTP and JSON handling

## License

MIT License 