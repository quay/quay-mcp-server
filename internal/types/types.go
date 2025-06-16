package types

// EndpointInfo stores information about a discovered API endpoint
type EndpointInfo struct {
	Method      string
	Path        string
	Summary     string
	OperationID string
	Tags        []string
	Parameters  []interface{}
}
