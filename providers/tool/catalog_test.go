package tool

import (
	"context"
	"sync"
	"testing"

	"github.com/leofalp/aigo/core/cost"
	"github.com/leofalp/aigo/providers/ai"
)

// mockTool is a simple mock implementation of GenericTool for testing
type mockTool struct {
	name   string
	result string
}

func (m *mockTool) ToolInfo() ai.ToolDescription {
	return ai.ToolDescription{
		Name:        m.name,
		Description: "Mock tool for testing",
		Parameters:  nil,
	}
}

func (m *mockTool) Call(ctx context.Context, inputJson string) (string, error) {
	return m.result, nil
}

func (m *mockTool) GetMetrics() *cost.ToolMetrics {
	return nil // Mock tool has no metrics
}

func TestNewCatalog(t *testing.T) {
	catalog := NewCatalog()
	if catalog == nil {
		t.Fatal("NewCatalog returned nil")
	}
	if catalog.Size() != 0 {
		t.Errorf("New catalog should be empty, got size %d", catalog.Size())
	}
}

func TestNewCatalogWithTools(t *testing.T) {
	tool1 := &mockTool{name: "tool1", result: "result1"}
	tool2 := &mockTool{name: "tool2", result: "result2"}

	catalog := NewCatalogWithTools(tool1, tool2)

	if catalog.Size() != 2 {
		t.Errorf("Expected catalog size 2, got %d", catalog.Size())
	}

	if !catalog.Has("tool1") {
		t.Error("Catalog should contain tool1")
	}
	if !catalog.Has("tool2") {
		t.Error("Catalog should contain tool2")
	}
}

func TestCatalog_AddTools_Single(t *testing.T) {
	catalog := NewCatalog()
	tool := &mockTool{name: "testTool", result: "result"}

	catalog.AddTools(tool)

	if catalog.Size() != 1 {
		t.Errorf("Expected size 1, got %d", catalog.Size())
	}

	retrieved, exists := catalog.Get("testTool")
	if !exists {
		t.Fatal("Tool should exist in catalog")
	}
	if retrieved != tool {
		t.Error("Retrieved tool is not the same as added tool")
	}
}

func TestCatalog_AddTools(t *testing.T) {
	catalog := NewCatalog()
	tool1 := &mockTool{name: "tool1", result: "result1"}
	tool2 := &mockTool{name: "tool2", result: "result2"}
	tool3 := &mockTool{name: "tool3", result: "result3"}

	catalog.AddTools(tool1, tool2, tool3)

	if catalog.Size() != 3 {
		t.Errorf("Expected size 3, got %d", catalog.Size())
	}

	for _, name := range []string{"tool1", "tool2", "tool3"} {
		if !catalog.Has(name) {
			t.Errorf("Catalog should contain %s", name)
		}
	}
}

func TestCatalog_Get(t *testing.T) {
	catalog := NewCatalog()
	tool := &mockTool{name: "testTool", result: "result"}
	catalog.AddTools(tool)

	// Test existing tool
	retrieved, exists := catalog.Get("testTool")
	if !exists {
		t.Fatal("Tool should exist")
	}
	if retrieved != tool {
		t.Error("Retrieved tool is not the expected tool")
	}

	// Test non-existing tool
	_, exists = catalog.Get("nonExistent")
	if exists {
		t.Error("Non-existent tool should not exist")
	}
}

func TestCatalog_GetCaseInsensitive(t *testing.T) {
	catalog := NewCatalog()
	tool := &mockTool{name: "TestTool", result: "result"}
	catalog.AddTools(tool)

	testCases := []struct {
		name       string
		queryName  string
		shouldFind bool
	}{
		{"exact match", "TestTool", true},
		{"lowercase", "testtool", true},
		{"uppercase", "TESTTOOL", true},
		{"mixed case", "tEsTtOoL", true},
		{"non-existent", "OtherTool", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			retrieved, exists := catalog.Get(tc.queryName)
			if exists != tc.shouldFind {
				t.Errorf("Get(%q): expected exists=%v, got %v", tc.queryName, tc.shouldFind, exists)
			}
			if tc.shouldFind && retrieved != tool {
				t.Error("Retrieved tool is not the expected tool")
			}
		})
	}
}

func TestCatalog_Has(t *testing.T) {
	catalog := NewCatalog()
	tool := &mockTool{name: "testTool", result: "result"}
	catalog.AddTools(tool)

	if !catalog.Has("testTool") {
		t.Error("Has should return true for existing tool")
	}

	if catalog.Has("nonExistent") {
		t.Error("Has should return false for non-existing tool")
	}

	// Has is case-insensitive
	if !catalog.Has("TESTTOOL") {
		t.Error("Has should be case-insensitive")
	}
}

func TestCatalog_Remove(t *testing.T) {
	catalog := NewCatalog()
	tool := &mockTool{name: "testTool", result: "result"}
	catalog.AddTools(tool)

	// Remove existing tool
	removed := catalog.Remove("testTool")
	if !removed {
		t.Error("Remove should return true when removing existing tool")
	}
	if catalog.Has("testTool") {
		t.Error("Tool should not exist after removal")
	}
	if catalog.Size() != 0 {
		t.Errorf("Catalog should be empty after removal, got size %d", catalog.Size())
	}

	// Remove non-existing tool
	removed = catalog.Remove("nonExistent")
	if removed {
		t.Error("Remove should return false for non-existing tool")
	}
}

func TestCatalog_Clear(t *testing.T) {
	catalog := NewCatalog()
	catalog.AddTools(
		&mockTool{name: "tool1", result: "result1"},
		&mockTool{name: "tool2", result: "result2"},
		&mockTool{name: "tool3", result: "result3"},
	)

	if catalog.Size() != 3 {
		t.Errorf("Expected size 3 before clear, got %d", catalog.Size())
	}

	catalog.Clear()

	if catalog.Size() != 0 {
		t.Errorf("Expected size 0 after clear, got %d", catalog.Size())
	}

	if catalog.Has("tool1") || catalog.Has("tool2") || catalog.Has("tool3") {
		t.Error("Catalog should not contain any tools after clear")
	}
}

func TestCatalog_Tools(t *testing.T) {
	catalog := NewCatalog()
	tool1 := &mockTool{name: "tool1", result: "result1"}
	tool2 := &mockTool{name: "tool2", result: "result2"}

	catalog.AddTools(tool1, tool2)

	tools := catalog.Tools()

	if len(tools) != 2 {
		t.Fatalf("Expected 2 tools, got %d", len(tools))
	}

	if tools["tool1"] != tool1 {
		t.Error("Tool1 mismatch")
	}
	if tools["tool2"] != tool2 {
		t.Error("Tool2 mismatch")
	}

	// Verify it's a copy (modifying returned map shouldn't affect catalog)
	delete(tools, "tool1")
	if !catalog.Has("tool1") {
		t.Error("Modifying returned map should not affect catalog")
	}
}

func TestCatalog_Size(t *testing.T) {
	catalog := NewCatalog()

	if catalog.Size() != 0 {
		t.Errorf("Empty catalog should have size 0, got %d", catalog.Size())
	}

	catalog.AddTools(&mockTool{name: "tool1", result: "result1"})
	if catalog.Size() != 1 {
		t.Errorf("After adding 1 tool, size should be 1, got %d", catalog.Size())
	}

	catalog.AddTools(&mockTool{name: "tool2", result: "result2"})
	if catalog.Size() != 2 {
		t.Errorf("After adding 2 tools, size should be 2, got %d", catalog.Size())
	}

	catalog.Remove("tool1")
	if catalog.Size() != 1 {
		t.Errorf("After removing 1 tool, size should be 1, got %d", catalog.Size())
	}
}

func TestCatalog_Merge(t *testing.T) {
	catalog1 := NewCatalog()
	catalog1.AddTools(
		&mockTool{name: "tool1", result: "result1"},
		&mockTool{name: "tool2", result: "result2"},
	)

	catalog2 := NewCatalog()
	catalog2.AddTools(
		&mockTool{name: "tool3", result: "result3"},
		&mockTool{name: "tool4", result: "result4"},
	)

	catalog1.Merge(catalog2)

	if catalog1.Size() != 4 {
		t.Errorf("After merge, catalog1 should have 4 tools, got %d", catalog1.Size())
	}

	for _, name := range []string{"tool1", "tool2", "tool3", "tool4"} {
		if !catalog1.Has(name) {
			t.Errorf("Catalog1 should contain %s after merge", name)
		}
	}

	// Verify catalog2 is unchanged
	if catalog2.Size() != 2 {
		t.Errorf("Catalog2 should remain unchanged with 2 tools, got %d", catalog2.Size())
	}
}

func TestCatalog_MergeWithOverride(t *testing.T) {
	catalog1 := NewCatalog()
	tool1v1 := &mockTool{name: "tool1", result: "result1_v1"}
	catalog1.AddTools(tool1v1)

	catalog2 := NewCatalog()
	tool1v2 := &mockTool{name: "tool1", result: "result1_v2"}
	catalog2.AddTools(tool1v2)

	catalog1.Merge(catalog2)

	if catalog1.Size() != 1 {
		t.Errorf("After merge with override, should have 1 tool, got %d", catalog1.Size())
	}

	retrieved, _ := catalog1.Get("tool1")
	if retrieved != tool1v2 {
		t.Error("Merge should override existing tool with the one from other catalog")
	}
}

func TestCatalog_MergeNil(t *testing.T) {
	catalog := NewCatalog()
	catalog.AddTools(&mockTool{name: "tool1", result: "result1"})

	catalog.Merge(nil)

	if catalog.Size() != 1 {
		t.Errorf("Merging nil should not change catalog, got size %d", catalog.Size())
	}
}

func TestCatalog_Clone(t *testing.T) {
	original := NewCatalog()
	tool1 := &mockTool{name: "tool1", result: "result1"}
	tool2 := &mockTool{name: "tool2", result: "result2"}
	original.AddTools(tool1, tool2)

	clone := original.Clone()

	// Verify clone has same content
	if clone.Size() != original.Size() {
		t.Errorf("Clone size %d doesn't match original size %d", clone.Size(), original.Size())
	}

	for _, name := range []string{"tool1", "tool2"} {
		if !clone.Has(name) {
			t.Errorf("Clone should contain %s", name)
		}
	}

	// Verify clone is independent
	clone.AddTools(&mockTool{name: "tool3", result: "result3"})
	if original.Has("tool3") {
		t.Error("Modifying clone should not affect original")
	}

	original.AddTools(&mockTool{name: "tool4", result: "result4"})
	if clone.Has("tool4") {
		t.Error("Modifying original should not affect clone")
	}
}

func TestCatalog_CaseInsensitiveStorage(t *testing.T) {
	catalog := NewCatalog()

	// Add tools with different casings - should replace each other
	catalog.AddTools(&mockTool{name: "tool1", result: "result1"})
	if catalog.Size() != 1 {
		t.Errorf("Expected size 1 after first add, got %d", catalog.Size())
	}

	catalog.AddTools(&mockTool{name: "Tool1", result: "result1_v2"})
	if catalog.Size() != 1 {
		t.Errorf("Expected size 1 after adding Tool1 (should replace tool1), got %d", catalog.Size())
	}

	catalog.AddTools(&mockTool{name: "TOOL1", result: "result1_v3"})
	if catalog.Size() != 1 {
		t.Errorf("Expected size 1 after adding TOOL1 (should replace Tool1), got %d", catalog.Size())
	}

	// Verify we can retrieve with any casing
	for _, name := range []string{"tool1", "Tool1", "TOOL1", "tOoL1"} {
		if !catalog.Has(name) {
			t.Errorf("Tool should exist for query '%s'", name)
		}
	}
}

func TestCatalog_Validate(t *testing.T) {
	catalog := NewCatalog()

	// No issues
	catalog.AddTools(
		&mockTool{name: "tool1", result: "result1"},
		&mockTool{name: "tool2", result: "result2"},
	)

	warnings := catalog.Validate()
	if warnings != nil {
		t.Errorf("Expected nil warnings for valid catalog, got %v", warnings)
	}

	// Add more tools - still no warnings since catalog is case-insensitive by design
	catalog.AddTools(&mockTool{name: "Tool1", result: "result1_v2"})

	warnings = catalog.Validate()
	if warnings != nil {
		t.Errorf("Expected nil warnings, got %v", warnings)
	}
}

func TestCatalog_ThreadSafety(t *testing.T) {
	catalog := NewCatalog()
	var wg sync.WaitGroup

	// Concurrent writes
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			name := "tool" + string(rune('A'+n%26))
			catalog.AddTools(&mockTool{name: name, result: "result"})
		}(i)
	}

	// Concurrent reads
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			name := "tool" + string(rune('A'+n%26))
			catalog.Has(name)
			catalog.Get(name)
			catalog.Tools()
			catalog.Size()
		}(i)
	}

	wg.Wait()

	// Verify catalog is still functional
	if catalog.Size() < 0 {
		t.Error("Catalog corrupted after concurrent operations")
	}
}

func TestCatalog_AddToolsReplacesExisting(t *testing.T) {
	catalog := NewCatalog()
	tool1v1 := &mockTool{name: "tool1", result: "result1_v1"}
	tool1v2 := &mockTool{name: "tool1", result: "result1_v2"}

	catalog.AddTools(tool1v1)

	retrieved, _ := catalog.Get("tool1")
	if retrieved != tool1v1 {
		t.Error("Should have v1 initially")
	}

	catalog.AddTools(tool1v2)

	retrieved, _ = catalog.Get("tool1")
	if retrieved != tool1v2 {
		t.Error("AddTools should replace existing tool")
	}

	if catalog.Size() != 1 {
		t.Errorf("Should still have 1 tool after replacement, got %d", catalog.Size())
	}
}
