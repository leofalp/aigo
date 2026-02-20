package graph

import (
	"context"
	"sync"
)

// StateProvider defines the interface for graph state persistence.
// It manages both shared state (arbitrary key-value data accessible to all nodes)
// and graph execution state (node statuses and results).
//
// The default implementation is InMemoryStateProvider, which stores everything
// in memory using sync.RWMutex for thread safety. State is lost when the
// process exits.
//
// Users can implement this interface to persist state to databases (PostgreSQL,
// Redis), file systems, or any other storage backend. This enables:
//   - Resuming partially completed graphs after process crashes
//   - Distributed execution across multiple processes
//   - Audit trails and debugging of state transitions
//   - Long-running workflows that survive process restarts
//
// All methods accept a context for cancellation and timeout support.
// Implementations must be safe for concurrent use by multiple goroutines,
// as nodes at the same level execute in parallel.
//
// The NodeResult.Output field must be JSON-serializable for implementations
// that persist to external storage. The InMemoryStateProvider does not
// impose this restriction.
type StateProvider interface {
	// Get retrieves a value from the shared state by key.
	// Returns the value, a boolean indicating whether the key exists, and any error.
	Get(ctx context.Context, key string) (any, bool, error)

	// Set writes a value to the shared state under the given key.
	// Overwrites any existing value for the same key.
	Set(ctx context.Context, key string, value any) error

	// GetAll retrieves the entire shared state as a map.
	// Returns a copy of the internal state to prevent external mutations.
	GetAll(ctx context.Context) (map[string]any, error)

	// GetNodeStatus retrieves the execution status of a node by its ID.
	// Returns NodePending if the node has not been registered yet.
	GetNodeStatus(ctx context.Context, nodeID string) (NodeStatus, error)

	// SetNodeStatus updates the execution status of a node.
	// Valid transitions: pending -> running -> completed|failed, pending -> skipped.
	SetNodeStatus(ctx context.Context, nodeID string, status NodeStatus) error

	// GetNodeResult retrieves the execution result of a node.
	// Returns nil if the node has not completed yet or if no result was stored.
	GetNodeResult(ctx context.Context, nodeID string) (*NodeResult, error)

	// SetNodeResult stores the execution result of a node.
	// The result is typically set when a node transitions to completed or failed.
	SetNodeResult(ctx context.Context, nodeID string, result *NodeResult) error
}

// InMemoryStateProvider is the default StateProvider implementation.
// It stores all state in memory using sync.RWMutex for thread safety.
// State is lost when the process exits.
//
// This provider is suitable for single-process, non-persistent workflows.
// For persistent or distributed workflows, implement a custom StateProvider
// backed by a database or distributed cache.
//
// Example:
//
//	provider := graph.NewInMemoryStateProvider(map[string]any{
//	    "profile_id": "user-123",
//	    "language":   "en",
//	})
type InMemoryStateProvider struct {
	mu          sync.RWMutex
	data        map[string]any
	nodeStatus  map[string]NodeStatus
	nodeResults map[string]*NodeResult
}

// Compile-time check that InMemoryStateProvider implements StateProvider.
var _ StateProvider = (*InMemoryStateProvider)(nil)

// NewInMemoryStateProvider creates a new in-memory state provider with optional
// initial shared state. If initial is nil, an empty state is created.
func NewInMemoryStateProvider(initial map[string]any) *InMemoryStateProvider {
	data := make(map[string]any)
	for key, value := range initial {
		data[key] = value
	}

	return &InMemoryStateProvider{
		data:        data,
		nodeStatus:  make(map[string]NodeStatus),
		nodeResults: make(map[string]*NodeResult),
	}
}

// Get retrieves a value from the shared state by key.
// Returns the value, true if found, and nil error (in-memory never fails).
func (provider *InMemoryStateProvider) Get(_ context.Context, key string) (any, bool, error) {
	provider.mu.RLock()
	defer provider.mu.RUnlock()

	value, exists := provider.data[key]
	return value, exists, nil
}

// Set writes a value to the shared state under the given key.
// Always returns nil error (in-memory never fails).
func (provider *InMemoryStateProvider) Set(_ context.Context, key string, value any) error {
	provider.mu.Lock()
	defer provider.mu.Unlock()

	provider.data[key] = value
	return nil
}

// GetAll retrieves the entire shared state as a copy of the internal map.
// The returned map is safe to modify without affecting the provider's state.
func (provider *InMemoryStateProvider) GetAll(_ context.Context) (map[string]any, error) {
	provider.mu.RLock()
	defer provider.mu.RUnlock()

	dataCopy := make(map[string]any, len(provider.data))
	for key, value := range provider.data {
		dataCopy[key] = value
	}
	return dataCopy, nil
}

// GetNodeStatus retrieves the execution status of a node.
// Returns NodePending if the node has not been registered.
func (provider *InMemoryStateProvider) GetNodeStatus(_ context.Context, nodeID string) (NodeStatus, error) {
	provider.mu.RLock()
	defer provider.mu.RUnlock()

	status, exists := provider.nodeStatus[nodeID]
	if !exists {
		return NodePending, nil
	}
	return status, nil
}

// SetNodeStatus updates the execution status of a node.
func (provider *InMemoryStateProvider) SetNodeStatus(_ context.Context, nodeID string, status NodeStatus) error {
	provider.mu.Lock()
	defer provider.mu.Unlock()

	provider.nodeStatus[nodeID] = status
	return nil
}

// GetNodeResult retrieves the execution result of a node.
// Returns nil if no result has been stored for this node.
func (provider *InMemoryStateProvider) GetNodeResult(_ context.Context, nodeID string) (*NodeResult, error) {
	provider.mu.RLock()
	defer provider.mu.RUnlock()

	result, exists := provider.nodeResults[nodeID]
	if !exists {
		return nil, nil
	}
	return result, nil
}

// SetNodeResult stores the execution result of a node.
func (provider *InMemoryStateProvider) SetNodeResult(_ context.Context, nodeID string, result *NodeResult) error {
	provider.mu.Lock()
	defer provider.mu.Unlock()

	provider.nodeResults[nodeID] = result
	return nil
}

// initializeNodes resets all node IDs to NodePending status and clears their results.
// This is called during graph execution initialization to ensure a clean state,
// including when re-executing a graph without an explicit Reset().
func (provider *InMemoryStateProvider) initializeNodes(nodeIDs []string) {
	provider.mu.Lock()
	defer provider.mu.Unlock()

	for _, nodeID := range nodeIDs {
		provider.nodeStatus[nodeID] = NodePending
		delete(provider.nodeResults, nodeID)
	}
}

// resetNodeState clears the status and result for a specific node,
// returning it to NodePending.
func (provider *InMemoryStateProvider) resetNodeState(nodeID string) {
	provider.mu.Lock()
	defer provider.mu.Unlock()

	provider.nodeStatus[nodeID] = NodePending
	delete(provider.nodeResults, nodeID)
}

// getAllNodeStatuses returns a snapshot of all node statuses.
// The returned map is a copy safe for concurrent reads.
func (provider *InMemoryStateProvider) getAllNodeStatuses() map[string]NodeStatus {
	provider.mu.RLock()
	defer provider.mu.RUnlock()

	statusCopy := make(map[string]NodeStatus, len(provider.nodeStatus))
	for nodeID, status := range provider.nodeStatus {
		statusCopy[nodeID] = status
	}
	return statusCopy
}
