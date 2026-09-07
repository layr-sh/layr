package core

import (
	"testing"
)

func TestCoreNodeRegistryNewUnit(t *testing.T) {
	nodeRegistry := NewNodeRegistry(nil, "test-node", []string{"data", "auth"})
	if nodeRegistry == nil {
		t.Fatal("expected non-nil NodeRegistry instance")
	}
	if nodeRegistry.nodeName != "test-node" {
		t.Errorf("expected nodeName test-node, got %s", nodeRegistry.nodeName)
	}
	if len(nodeRegistry.services) != 2 {
		t.Errorf("expected 2 services, got %d", len(nodeRegistry.services))
	}

	// Clean close of unstarted registry (idempotent double-close test)
	nodeRegistry.Close()
	nodeRegistry.Close()
}
