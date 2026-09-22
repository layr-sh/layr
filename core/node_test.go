package core

import (
	"testing"
)

func TestCoreNodeNewUnit(t *testing.T) {
	node := NewNode(nil, "test-node", []string{"data", "auth"})
	if node.nodeName != "test-node" {
		t.Errorf("expected nodeName test-node, got %s", node.nodeName)
	}
	if len(node.services) != 2 {
		t.Errorf("expected 2 services, got %d", len(node.services))
	}

	// Clean close of unstarted registry (idempotent double-close test)
	node.Close()
	node.Close()
}
