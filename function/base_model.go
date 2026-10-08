// Package function defines the serverless and edge function execution engine.
package function

import (
	"errors"
)

// Standard sentinel error for base data plane invocation.
var (
	ErrNoActiveDeployment = errors.New("no active deployment")
)

// InvokeEndpointInput represents dynamic caller input payload passed to a function invocation.
type InvokeEndpointInput any

// InvokeEndpointResponse represents dynamic output payload returned from a function invocation.
type InvokeEndpointResponse any
