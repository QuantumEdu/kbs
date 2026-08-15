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
	loopDetector    *LoopDetector
	stallDetector   *StallDetector
	streakDetector    *StreakDetector
	tokenCounter      *TokenCounter
	injectionDetector *InjectionDetector
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

// SetLoopDetector attaches a loop detector to the collector.
func (c *Collector) SetLoopDetector(ld *LoopDetector) {
	c.loopDetector = ld
}

// SetStallDetector attaches a stall detector to the collector.
func (c *Collector) SetStallDetector(sd *StallDetector) {
	c.stallDetector = sd
}

// SetStreakDetector attaches a streak detector to the collector.
func (c *Collector) SetStreakDetector(sd *StreakDetector) {
	c.streakDetector = sd
}

// SetTokenCounter attaches a token counter to the collector.
func (c *Collector) SetTokenCounter(tc *TokenCounter) {
	c.tokenCounter = tc
}

// SetInjectionDetector attaches an injection detector to the collector.
func (c *Collector) SetInjectionDetector(id *InjectionDetector) {
	c.injectionDetector = id
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

	// Run quality signal detectors after successful save.

	// Loop detector: check tool.called events.
	if e.EventType == "tool.called" && c.loopDetector != nil {
		var tcr ToolCallRecord
		if json.Unmarshal(e.Payload, &tcr) == nil {
			if signal := c.loopDetector.Check(tcr); signal != nil {
				emitSignalEvent(c.store, ctx, e, "loop.detected", signal)
			}
		}
	}

	// Stall detector: record activity on every event, then check.
	if c.stallDetector != nil {
		c.stallDetector.Record(e.RunID, time.Now())
		if signal := c.stallDetector.Check(e.RunID); signal != nil {
			// Only fire for active (running) runs.
			run, err := c.store.GetRun(ctx, e.RunID)
			if err == nil && run.Status == "running" {
				emitSignalEvent(c.store, ctx, e, "policy.violation", signal)
			}
		}
	}

	// Streak detector: check tool status for failures.
	if e.EventType == "tool.called" && c.streakDetector != nil {
		var tcr ToolCallRecord
		if json.Unmarshal(e.Payload, &tcr) == nil {
			if tcr.ErrorType != "" {
				if signal := c.streakDetector.RecordFail(e.RunID); signal != nil {
					emitSignalEvent(c.store, ctx, e, "policy.violation", signal)
				}
			} else {
				c.streakDetector.RecordSuccess(e.RunID)
			}
		}
	}

	// Injection detector: check for prompt injection or command hazards.
	if c.injectionDetector != nil {
		if signal := c.injectionDetector.Check(e); signal != nil {
			emitSignalEvent(c.store, ctx, e, "policy.violation", signal)
		}
	}

	return AckOK
}

// emitSignalEvent marshals a signal payload and saves it as an event.
func emitSignalEvent(store *Store, ctx context.Context, source Event, eventType string, signal interface{}) {
	payload, err := json.Marshal(signal)
	if err != nil {
		return
	}
	signalEvent := Event{
		EventID:         fmt.Sprintf("evt-%d", time.Now().UnixNano()),
		EventType:       eventType,
		Timestamp:       time.Now(),
		RunID:           source.RunID,
		AgentID:         source.AgentID,
		Source:          "daemon",
		RedactionPolicy: "none",
		ConfidenceLevel: "measured",
		Payload:         payload,
	}
	_ = store.SaveEvent(ctx, signalEvent)
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
