package core

import (
	"context"
	"fmt"
	"sync"
	"time"
	"uuid"
)

// Node represents an active cluster node entity in core.nodes.
type Node struct {
	ID              uuid.UUID `json:"id"`
	NodeName        string    `json:"node_name"`
	EnabledServices []string  `json:"enabled_services"`
	StartedAt       time.Time `json:"started_at"`
	LastHeartbeatAt time.Time `json:"last_heartbeat_at"`
}

// NodeManager handles cluster heartbeats and active nodes.
type NodeManager struct {
	db                *DatabasePool
	eventBus          *EventBus
	node              Node
	nodeName          string
	services          []string
	stopChannel       chan struct{}
	closeOnce         sync.Once
	waitGroup         sync.WaitGroup
	heartbeatInterval time.Duration // configurable for testing; default 10s
	reaperInterval    time.Duration // configurable for testing; default 60s
}

// NewNodeManager initializes node in core.nodes.
func NewNodeManager(db *DatabasePool, nodeName string, services []string) *NodeManager {
	return &NodeManager{
		db:                db,
		nodeName:          nodeName,
		services:          services,
		stopChannel:       make(chan struct{}),
		heartbeatInterval: 10 * time.Second,
		reaperInterval:    60 * time.Second,
	}
}

// WithEventBus attaches an EventBus to the NodeManager for publishing cluster lifecycle events.
func (nodeManager *NodeManager) WithEventBus(eventBus *EventBus) *NodeManager {
	nodeManager.eventBus = eventBus
	return nodeManager
}

// Node returns the registered node entity.
func (nodeManager *NodeManager) Node() Node {
	return nodeManager.node
}

// NodeID returns the unique identifier of the node.
func (nodeManager *NodeManager) NodeID() uuid.UUID {
	return nodeManager.node.ID
}

const (
	heartbeatTimeoutDuration  = 5 * time.Second
	unregisterTimeoutDuration = 3 * time.Second
)

// Register registers this node and starts background heartbeat.
func (nodeManager *NodeManager) Register(ctx context.Context) error {
	log.Debugf("registering node %s in cluster", nodeManager.nodeName)
	var node Node
	err := nodeManager.db.QueryRow(ctx, `
		INSERT INTO core.nodes (node_name, enabled_services, started_at, last_heartbeat_at)
		VALUES ($1, $2, clock_timestamp(), clock_timestamp())
		RETURNING id, node_name, enabled_services, started_at, last_heartbeat_at
	`, nodeManager.nodeName, nodeManager.services).Scan(
		&node.ID,
		&node.NodeName,
		&node.EnabledServices,
		&node.StartedAt,
		&node.LastHeartbeatAt,
	)
	if err != nil {
		return fmt.Errorf("failed to register node in core.nodes: %w", err)
	}
	nodeManager.node = node

	log.Tracef("node %s registered with id %s", node.NodeName, node.ID)
	if nodeManager.eventBus != nil {
		nodeManager.eventBus.Publish(ctx, NewNodeRegisteredEvent(node.ID.String(), NodeRegisteredEventData(node)))
	}
	nodeManager.waitGroup.Add(1)
	go nodeManager.startHeartbeatLoop(ctx)
	return nil
}

func (nodeManager *NodeManager) startHeartbeatLoop(ctx context.Context) {
	defer nodeManager.waitGroup.Done()
	ticker := time.NewTicker(nodeManager.heartbeatInterval)
	defer ticker.Stop()
	lastReapedAt := time.Now()

	for {
		select {
		case <-ticker.C:
			// Heartbeats must outlive transient request cancellation; detach but keep values.
			{
				heartbeatCtx, heartbeatCancel := context.WithTimeout(context.WithoutCancel(ctx), heartbeatTimeoutDuration)
				if _, err := nodeManager.db.Exec(heartbeatCtx, "UPDATE core.nodes SET last_heartbeat_at = clock_timestamp() WHERE id = $1", nodeManager.node.ID); err != nil {
					log.Warnf("failed to update node heartbeat: %v", err)
				}
				if time.Since(lastReapedAt) >= nodeManager.reaperInterval {
					lastReapedAt = time.Now()
					if _, err := nodeManager.db.Exec(heartbeatCtx, "DELETE FROM core.nodes WHERE last_heartbeat_at < clock_timestamp() - INTERVAL '60 seconds'"); err != nil {
						log.Warnf("failed to reap stale nodes: %v", err)
					}
				}
				heartbeatCancel()
			}

		case <-nodeManager.stopChannel:
			// Unregister must run even though stopChannel closed; detach from ctx.
			{
				unregisterCtx, unregisterCancel := context.WithTimeout(context.WithoutCancel(ctx), unregisterTimeoutDuration)
				if _, err := nodeManager.db.Exec(unregisterCtx, "DELETE FROM core.nodes WHERE id = $1", nodeManager.node.ID); err != nil {
					log.Warnf("failed to unregister node: %v", err)
				}
				if nodeManager.eventBus != nil {
					nodeManager.eventBus.Publish(unregisterCtx, NewNodeUnregisteredEvent(nodeManager.node.ID.String(), NodeUnregisteredEventData(nodeManager.node)))
				}
				unregisterCancel()
			}
			return
		}
	}
}

// Close stops heartbeat and unregisters the node.
func (nodeManager *NodeManager) Close() {
	nodeManager.closeOnce.Do(func() {
		log.Debugf("closing node registry for node %s", nodeManager.nodeName)
		close(nodeManager.stopChannel)
		nodeManager.waitGroup.Wait()
		log.Tracef("node registry closed for node %s", nodeManager.nodeName)
	})
}
