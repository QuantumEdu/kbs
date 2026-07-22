package agenttelemetry

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"sync"
	"time"
)

// AckOK is the JSON acknowledgment for a successfully ingested event.
const AckOK = `{"status":"ok"}` + "\n"

// Collector listens on a Unix socket and dispatches events to validation and storage.
type Collector struct {
	store           *Store
	security        *SecurityPipeline
	socketPath      string
	dbPath          string
	promptStorage   bool
	daemonStartTime time.Time
	listener        net.Listener
	mu              sync.Mutex
	closed          bool
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

// SetDaemonStartTime records when the daemon started, used for uptime
// calculation in the status command.
func (c *Collector) SetDaemonStartTime(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.daemonStartTime = t
}

// SetDBPath sets the path to the SQLite database file, used for DB size
// reporting in the status command.
func (c *Collector) SetDBPath(path string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.dbPath = path
}

// SetPromptStorage records whether prompt storage is enabled.
func (c *Collector) SetPromptStorage(enabled bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.promptStorage = enabled
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
	// Check if this is a command message (has "cmd" field instead of "event_type").
	var cmdMsg struct {
		Cmd string `json:"cmd"`
	}
	if json.Unmarshal(raw, &cmdMsg) == nil && cmdMsg.Cmd != "" {
		return c.handleCommand(ctx, cmdMsg.Cmd)
	}

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

// handleCommand processes a command message from a client.
func (c *Collector) handleCommand(ctx context.Context, cmd string) string {
	switch cmd {
	case "status":
		return c.statusCommand(ctx)
	default:
		return `{"status":"error","error":"unknown command"}` + "\n"
	}
}

// statusCommand builds and returns the daemon status JSON response.
func (c *Collector) statusCommand(ctx context.Context) string {
	c.mu.Lock()
	dbPath := c.dbPath
	promptStorage := c.promptStorage
	startTime := c.daemonStartTime
	security := c.security
	c.mu.Unlock()

	ds, err := c.store.Status(ctx)
	if err != nil {
		return fmt.Sprintf(`{"status":"error","error":%q}`+"\n", err.Error())
	}

	uptime := int64(0)
	if !startTime.IsZero() {
		uptime = int64(time.Since(startTime).Seconds())
	}

	var dbSize int64
	if dbPath != "" {
		if info, statErr := os.Stat(dbPath); statErr == nil {
			dbSize = info.Size()
		}
	}

	saltFp := ""
	var patterns []string
	if security != nil {
		saltFp = security.SaltFingerprint()
		patterns = security.RedactionPatterns()
	}

	data := map[string]interface{}{
		"uptime_seconds":     uptime,
		"events_ingested":    ds.EventsIngested,
		"db_size_bytes":      dbSize,
		"salt_fingerprint":   saltFp,
		"redaction_patterns": patterns,
		"prompt_storage":     promptStorage,
	}

	resp := map[string]interface{}{
		"status": "ok",
		"data":   data,
	}

	b, _ := json.Marshal(resp)
	return string(b) + "\n"
}
