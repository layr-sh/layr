package core

import (
	"context"
	"fmt"
	"sync"
	"time"
	"uuid"
)

// Node handles cluster heartbeats and active nodes.
type Node struct {
	db                *DatabasePool
	nodeID            uuid.UUID
	nodeName          string
	services          []string
	stopChannel       chan struct{}
	closeOnce         sync.Once
	waitGroup         sync.WaitGroup
	heartbeatInterval time.Duration // configurable for testing; default 10s
	reaperInterval    time.Duration // configurable for testing; default 60s
}

// NewNode initializes node in core.nodes.
func NewNode(db *DatabasePool, nodeName string, services []string) *Node {
	return &Node{
		db:                db,
		nodeName:          nodeName,
		services:          services,
		stopChannel:       make(chan struct{}),
		heartbeatInterval: 10 * time.Second,
		reaperInterval:    60 * time.Second,
	}
}

const (
	heartbeatTimeoutDuration  = 5 * time.Second
	unregisterTimeoutDuration = 3 * time.Second
)

// Register registers this node and starts background heartbeat.
func (node *Node) Register(ctx context.Context) error {
	log.Debugf("registering node %s in cluster", node.nodeName)
	var nodeID uuid.UUID
	err := node.db.QueryRow(ctx, `
		INSERT INTO core.nodes (node_name, enabled_services, last_heartbeat_at)
		VALUES ($1, $2, clock_timestamp())
		RETURNING id
	`, node.nodeName, node.services).Scan(&nodeID)
	if err != nil {
		return fmt.Errorf("failed to register node in core.nodes: %w", err)
	}
	node.nodeID = nodeID

	log.Tracef("node %s registered with id %s", node.nodeName, nodeID)
	node.waitGroup.Add(1)
	go node.startHeartbeatLoop(ctx)
	return nil
}

func (node *Node) startHeartbeatLoop(ctx context.Context) {
	defer node.waitGroup.Done()
	ticker := time.NewTicker(node.heartbeatInterval)
	defer ticker.Stop()
	lastReapedAt := time.Now()

	for {
		select {
		case <-ticker.C:
			// Heartbeats must outlive transient request cancellation; detach but keep values.
			{
				heartbeatCtx, heartbeatCancel := context.WithTimeout(context.WithoutCancel(ctx), heartbeatTimeoutDuration)
				if _, err := node.db.Exec(heartbeatCtx, "UPDATE core.nodes SET last_heartbeat_at = clock_timestamp() WHERE id = $1", node.nodeID); err != nil {
					log.Warnf("failed to update node heartbeat: %v", err)
				}
				if time.Since(lastReapedAt) >= node.reaperInterval {
					lastReapedAt = time.Now()
					if _, err := node.db.Exec(heartbeatCtx, "DELETE FROM core.nodes WHERE last_heartbeat_at < clock_timestamp() - INTERVAL '60 seconds'"); err != nil {
						log.Warnf("failed to reap stale nodes: %v", err)
					}
				}
				heartbeatCancel()
			}

		case <-node.stopChannel:
			// Unregister must run even though stopChannel closed; detach from ctx.
			{
				unregisterCtx, unregisterCancel := context.WithTimeout(context.WithoutCancel(ctx), unregisterTimeoutDuration)
				if _, err := node.db.Exec(unregisterCtx, "DELETE FROM core.nodes WHERE id = $1", node.nodeID); err != nil {
					log.Warnf("failed to unregister node: %v", err)
				}
				unregisterCancel()
			}
			return
		}
	}
}

// Close stops heartbeat and unregisters the node.
func (node *Node) Close() {
	node.closeOnce.Do(func() {
		log.Debugf("closing node registry for node %s", node.nodeName)
		close(node.stopChannel)
		node.waitGroup.Wait()
		log.Tracef("node registry closed for node %s", node.nodeName)
	})
}
