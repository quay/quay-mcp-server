# Quay MCP Server

A Model Context Protocol (MCP) server that provides access to Quay container registry APIs. This server automatically discovers Quay API endpoints from the OpenAPI specification and generates MCP tools for interacting with Quay registries.

## Project Structure

This project follows Go best practices with a clean, modular architecture:

```
quay-mcp-server/
├── cmd/
│   └── quay-mcp/           # Main application
│       └── main.go
├── internal/               # Private application code
│   ├── client/            # Quay API client
│   │   └── quay_client.go
│   ├── server/            # MCP server implementation
│   │   └── mcp_server.go
│   └── types/             # Common data types
│       └── types.go
├── examples/              # Example code and demonstrations
│   └── example.go
├── test/                  # Test files
│   └── main_test.go
├── testing/               # Test data and fixtures
├── bin/                   # Built binaries (created by build)
├── go.mod                 # Go module definition
├── go.sum                 # Go module checksums
├── Makefile              # Build automation
├── README.md             # This file
└── .gitignore            # Git ignore rules
```

## Features

- **Automatic API Discovery**: Fetches and parses Quay's OpenAPI specification
- **Dynamic Tool Generation**: Creates MCP tools from API endpoints
- **Smart Parameter Handling**: Supports both path and query parameters
- **Comprehensive Logging**: Detailed request/response logging with security features
- **Tag-based Filtering**: Only exposes relevant API endpoints (manifest, organization, repository, robot, tag)
- **Authentication Support**: OAuth token authentication for protected resources

## Installation

### Prerequisites

- Go 1.23 or later
- Access to a Quay registry (e.g., quay.io)

### Building from Source

```bash
# Clone the repository
git clone https://github.com/quay/quay-mcp-server.git
cd quay-mcp-server

# Install dependencies
make deps

# Build the application
make build

# The binary will be available at ./bin/quay-mcp
```

### Using Make Commands

```bash
# Build the application
make build

# Run tests
make test

# Run example mode
make run-example

# Clean build artifacts
make clean

# Format code
make fmt

# Install to GOPATH/bin
make install

# Show all available commands
make help
```

## Usage

### Basic Usage

```bash
# Start MCP server for quay.io
./bin/quay-mcp -url https://quay.io

# Start with OAuth token for authenticated access
./bin/quay-mcp -url https://quay.io -token your-oauth-token

# Run in example mode to see available tools
./bin/quay-mcp -url https://quay.io -example
```

### Command Line Options

- `-url <registry-url>`: Quay registry URL (required)
- `-token <oauth-token>`: OAuth token for authentication (optional)
- `-example`: Run in example mode to demonstrate functionality

### Integration with Claude Desktop

Add the following to your Claude Desktop MCP configuration:

```json
{
  "mcpServers": {
    "quay": {
      "command": "/path/to/bin/quay-mcp",
      "args": ["-url", "https://quay.io"],
      "env": {
        "QUAY_OAUTH_TOKEN": "your-oauth-token-here"
      }
    }
  }
}
```

### Integration with MCPHost

[MCPHost](https://github.com/mark3labs/mcphost) is a powerful CLI host application that enables Large Language Models (LLMs) to interact with MCP servers. You can use it to interact with your Quay registry through various LLM providers.

#### Installation

Install MCPHost using one of these methods:

```bash
# Using Go
go install github.com/mark3labs/mcphost@latest

# Using npm
npm install -g @mark3labs/mcphost

# Or download from releases
# https://github.com/mark3labs/mcphost/releases
```

#### Configuration

Create or update your MCPHost configuration file at `~/.mcphost.yml`:

```yaml
# MCP Servers configuration
mcpServers:
  quay:
    command: "/path/to/bin/quay-mcp"
    args: ["-url", "https://quay.io"]
    env:
      QUAY_OAUTH_TOKEN: "your-oauth-token-here"

# Application settings
model: "anthropic:claude-3-5-sonnet-latest"  # or openai:gpt-4, ollama:qwen2.5:3b, etc.
max-steps: 20
debug: false

# Model generation parameters
max-tokens: 4096
temperature: 0.7
top-p: 0.95
```

#### Usage Examples

**Interactive Mode:**
```bash
# Start interactive session with Claude
mcphost

# Use with different LLM providers
mcphost -m openai:gpt-4
mcphost -m ollama:qwen2.5:3b
mcphost -m google:gemini-2.0-flash
```

**Non-Interactive Mode (Perfect for Automation):**
```bash
# Single query with full UI
mcphost -p "List all repositories in the redhat namespace"

# Quiet mode for scripting (only AI response)
mcphost -p "Show manifest details for redhat/ubi8" --quiet

# Use in shell scripts
REPOS=$(mcphost -p "Get public repositories from quay.io/redhat" --quiet)
echo "Found repositories: $REPOS"
```

**Advanced Usage:**
```bash
# Custom parameters for more focused responses
mcphost -p "Analyze repository security tags" --temperature 0.3 --max-tokens 2000

# With custom stop sequences
mcphost -p "Generate automation script" --stop-sequences "```","END"
```

#### Interactive Commands

Once in MCPHost interactive mode, you can use:

- `/tools` - List all available Quay tools
- `/servers` - Show configured MCP servers  
- `/help` - Show available commands
- `/history` - Display conversation history
- `/quit` - Exit the application

#### Example Interactions

```
$ mcphost
🤖 MCPHost Interactive Mode
📡 Connected to Quay MCP Server

You: List repositories in the redhat namespace that are public

🤖 I'll help you list the public repositories in the redhat namespace using the Quay API.

[Uses quay_listRepos tool with namespace=redhat, public=true]

Here are the public repositories in the redhat namespace:
- redhat/ubi8: Red Hat Universal Base Image 8
- redhat/ubi9: Red Hat Universal Base Image 9  
- redhat/3scale-toolbox: 3scale toolbox container
...

You: Show me the manifest details for redhat/ubi8:latest

🤖 I'll retrieve the manifest details for the redhat/ubi8:latest image.

[Uses quay_getRepoManifest tool]

Here are the manifest details for redhat/ubi8:latest:
- Architecture: amd64
- Size: 234MB
- Layers: 3
- Digest: sha256:abc123...
...
```

#### Automation & Scripting

MCPHost's non-interactive mode makes it perfect for DevOps automation:

```bash
#!/bin/bash
# Repository monitoring script
REPOS=$(mcphost -p "List all repositories in namespace 'myorg' that haven't been updated in 30 days" --quiet)

# Security scanning
VULNS=$(mcphost -p "Check for security vulnerabilities in myorg/myapp:latest" --quiet)

# Automated reporting
mcphost -p "Generate a weekly report of repository activity for myorg namespace" --quiet > weekly-report.md
```

## API Coverage

The server automatically filters and exposes Quay API endpoints with the following tags:

- **manifest**: Container manifest operations
- **organization**: Organization management
- **repository**: Repository operations
- **robot**: Robot account management
- **tag**: Container tag operations

## Architecture

### Internal Packages

- **`internal/client`**: Quay API client with comprehensive HTTP handling, parameter processing, and authentication
- **`internal/server`**: MCP server implementation with tool registration and request handling
- **`internal/types`**: Common data structures used across packages

### Key Components

1. **Swagger Spec Discovery**: Automatically fetches OpenAPI specification from Quay
2. **Endpoint Discovery**: Parses specification to identify relevant API endpoints
3. **Tool Generation**: Creates MCP tools with proper parameter schemas
4. **Request Processing**: Handles both path parameters and query parameters
5. **Response Formatting**: Returns JSON responses with proper formatting

## Development

### Project Layout

The project follows the [Standard Go Project Layout](https://github.com/golang-standards/project-layout):

- `cmd/`: Main applications
- `internal/`: Private application and library code
- `examples/`: Examples and demonstrations
- `test/`: Additional external test apps and test data

### Adding New Features

1. **Client Features**: Add to `internal/client/quay_client.go`
2. **Server Features**: Add to `internal/server/mcp_server.go`
3. **Types**: Add common types to `internal/types/types.go`

### Testing

```bash
# Run all tests
make test

# Run tests with verbose output
go test -v ./...
```

## Logging

The server provides comprehensive logging including:

- API request/response details
- Parameter processing
- Authentication (with token masking for security)
- Error handling
- Performance metrics

## Security

- OAuth tokens are masked in logs for security
- Response bodies are truncated to prevent log overflow
- Internal packages are not exposed to external consumers

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes following Go best practices
4. Add tests for new functionality
5. Run `make fmt` and `make lint`
6. Submit a pull request

## License

This project is licensed under the Apache License 2.0 - see the LICENSE file for details.

## Support

For issues and questions:

1. Check the existing issues on GitHub
2. Create a new issue with detailed information
3. Include logs and reproduction steps

## Changelog

### v1.0.0
- Initial release with refactored architecture
- Proper Go project structure
- Comprehensive API coverage
- Full parameter support (path and query)
- Authentication and logging features 