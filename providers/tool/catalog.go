package tool

import (
	"strings"
	"sync"
)

// Catalog manages a collection of tools with thread-safe operations.
// It provides methods for adding, retrieving, and managing tools by name.
type Catalog struct {
	mu    sync.RWMutex
	tools map[string]GenericTool
}

// NewCatalog creates a new empty tool catalog.
func NewCatalog() *Catalog {
	return &Catalog{
		tools: make(map[string]GenericTool),
	}
}

// NewCatalogWithTools creates a new catalog pre-populated with the given tools.
// Tool names are taken from each tool's ToolInfo().Name.
func NewCatalogWithTools(tools ...GenericTool) *Catalog {
	catalog := NewCatalog()
	catalog.AddTools(tools...)
	return catalog
}

// AddTools adds multiple tools to the catalog.
// Tool names are automatically extracted from each tool's ToolInfo().Name and stored in lowercase.
// If a tool with the same name already exists, it will be replaced.
func (c *Catalog) AddTools(tools ...GenericTool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, t := range tools {
		info := t.ToolInfo()
		c.tools[strings.ToLower(info.Name)] = t
	}
}

// Get retrieves a tool by name (case-insensitive).
// Returns the tool and true if found, nil and false otherwise.
func (c *Catalog) Get(name string) (GenericTool, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	tool, exists := c.tools[strings.ToLower(name)]
	return tool, exists
}

// Has checks if a tool with the given name exists (case-insensitive).
func (c *Catalog) Has(name string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, exists := c.tools[strings.ToLower(name)]
	return exists
}

// Remove removes a tool from the catalog by name (case-insensitive).
// Returns true if the tool was found and removed, false otherwise.
func (c *Catalog) Remove(name string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	lowerName := strings.ToLower(name)
	if _, exists := c.tools[lowerName]; exists {
		delete(c.tools, lowerName)
		return true
	}
	return false
}

// Clear removes all tools from the catalog.
func (c *Catalog) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.tools = make(map[string]GenericTool)
}

// Tools returns a copy of the internal tool map.
// The returned map can be safely modified without affecting the catalog.
func (c *Catalog) Tools() map[string]GenericTool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	catalogCopy := make(map[string]GenericTool, len(c.tools))
	for name, tool := range c.tools {
		catalogCopy[name] = tool
	}
	return catalogCopy
}

// Size returns the number of tools in the catalog.
func (c *Catalog) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.tools)
}

// Merge adds all tools from another catalog into this one.
// If a tool with the same name already exists, it will be replaced with the one from 'other'.
func (c *Catalog) Merge(other *Catalog) {
	if other == nil {
		return
	}

	// Lock both catalogs (in consistent order to prevent deadlock)
	c.mu.Lock()
	defer c.mu.Unlock()

	other.mu.RLock()
	defer other.mu.RUnlock()

	for name, tool := range other.tools {
		c.tools[name] = tool
	}
}

// Clone creates a deep copy of the catalog.
// The returned catalog is independent and can be modified without affecting the original.
func (c *Catalog) Clone() *Catalog {
	c.mu.RLock()
	defer c.mu.RUnlock()

	clone := NewCatalog()
	for name, tool := range c.tools {
		clone.tools[name] = tool
	}
	return clone
}

// Validate checks the catalog for potential issues and returns any warnings.
// Since the catalog is case-insensitive by design, this is provided for future extensibility.
func (c *Catalog) Validate() []string {
	// Currently no validation needed since names are normalized to lowercase
	// This method is kept for API stability and future validation rules
	return nil
}
