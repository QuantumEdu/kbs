package agenttelemetry

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"sync"
)

// AckOK is the JSON acknowledgment for a successfully ingested event.
const AckOK = `{"status":"ok"}` + "\n"

// Collector listens on a Unix socket and dispatches events to validation and storage.
type Collector struct {
	store    *Store
	security *SecurityPipeline
	socketPath string
	listener   net.Listener
	mu         sync.Mutex
	closed     bool
}

// NewCollector creates a new Collector bound to the given socket path.
func NewCollector(store *Store, socketPath string) *Collector {
	return &Collector{
		store:      store,
		socketPath: socketPath,
	}
}

// SetSecurityPipeline attaches a security pipeline to the collector. When set,
// every ingested event passes through redaction and entropy scanning before
// storage. Passing nil clears the pipeline.
func (c *Collector) SetSecurityPipeline(sp *SecurityPipeline) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.security = sp
}

// Listen binds the Unix socket and accepts connections. Each connection is handled
// in its own goroutine. Returns when ctx is cancelled or the listener errors.
func (c *Collector) Listen(ctx context.Context) error {
	// Remove stale socket.
	_ = os.Remove(c.socketPath)

	l, err := net.Listen("unix", c.socketPath)
	if err != nil {
		return fmt.Errorf("listen unix %s: %w", c.socketPath, err)
	}
	c.mu.Lock()
	c.listener = l
	c.mu.Unlock()

	go func() {
		<-ctx.Done()
		l.Close()
	}()

	for {
		conn, err := l.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return nil
			default:
				return fmt.Errorf("accept: %w", err)
			}
		}
		go c.handleConn(ctx, conn)
	}
}

// handleConn reads line-delimited JSON from a client connection.
func (c *Collector) handleConn(ctx context.Context, conn net.Conn) {
	defer conn.Close()

	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		ack := c.ingest(ctx, line)
		if _, err := conn.Write([]byte(ack)); err != nil {
			return
		}
	}
}

// ingest validates and stores a raw event line. Returns the ack line.
func (c *Collector) ingest(ctx context.Context, raw []byte) string {
	if err := ValidateRaw(raw); err != nil {
		return fmt.Sprintf(`{"status":"error","error":%q}`+"\n", err.Error())
	}

	var e Event
	if err := json.Unmarshal(raw, &e); err != nil {
		return fmt.Sprintf(`{"status":"error","error":%q}`+"\n", err.Error())
	}

	if c.security != nil {
		c.security.Process(&e)
	}

	if err := c.store.SaveEvent(ctx, e); err != nil {
		return fmt.Sprintf(`{"status":"error","error":%q}`+"\n", err.Error())
	}

	return AckOK
}

// Shutdown drains pending connections and closes the socket.
func (c *Collector) Shutdown(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true

	if c.listener != nil {
		c.listener.Close()
	}
	_ = os.Remove(c.socketPath)
	return nil
}
