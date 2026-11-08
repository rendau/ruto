package grpc

import (
	"fmt"
	"sync"

	gogrpc "google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Conns is the pool of client connections to the gRPC backends, one per
// address. It is owned by the caller and outlives the handler: a pool per
// snapshot would leave the connections of the previous one open after every
// config change.
type Conns struct {
	mu    sync.Mutex
	items map[string]*gogrpc.ClientConn
}

func NewConns() *Conns {
	return &Conns{items: make(map[string]*gogrpc.ClientConn)}
}

// Get returns the connection to target, opening it on first use.
// gogrpc.NewClient does not dial, so holding the lock while it runs is cheap.
func (c *Conns) Get(target string) (*gogrpc.ClientConn, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if conn, ok := c.items[target]; ok {
		return conn, nil
	}

	conn, err := gogrpc.NewClient(target, gogrpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("gogrpc.NewClient: %w", err)
	}
	c.items[target] = conn

	return conn, nil
}

// Retain closes the connections to every backend not in targets: after a
// config change the backends that left it are dropped, in-flight calls to
// them included.
func (c *Conns) Retain(targets map[string]struct{}) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for target, conn := range c.items {
		if _, ok := targets[target]; ok {
			continue
		}
		_ = conn.Close()
		delete(c.items, target)
	}
}

// Close closes every connection; the pool is empty afterwards.
func (c *Conns) Close() {
	c.Retain(nil)
}
