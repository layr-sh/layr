package core

import (
	"testing"
)

func TestCoreNodeNewUnit(t *testing.T) {
	nodeManager := NewNodeManager(nil, "test-node", []string{"data", "auth"})
	if nodeManager.nodeName != "test-node" {
		t.Errorf("expected nodeName test-node, got %s", nodeManager.nodeName)
	}
	if len(nodeManager.services) != 2 {
		t.Errorf("expected 2 services, got %d", len(nodeManager.services))
	}

	eventBus := NewEventBus(nil, nil)
	nodeManager.WithEventBus(eventBus)
	if nodeManager.eventBus != eventBus {
		t.Fatal("expected eventBus to be set on node")
	}

	// Test Node and NodeID getters
	if nodeManager.Node().ID != nodeManager.NodeID() {
		t.Errorf("expected nodeManager.Node().ID == nodeManager.NodeID()")
	}

	// Clean close of unstarted registry (idempotent double-close test)
	nodeManager.Close()
	nodeManager.Close()
}
