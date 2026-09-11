package line

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const usageText = `line is a deterministic issue-to-PR factory for the kbs suite.

Phases: plan -> build -> review -> wait_ci -> ready.
CI failure parks at repair (max 3), then review again.

Usage:
  line run [--repo <path>] [--exec claude] [--exec-arg --print] [--ntfy-topic T] <issue-url>
  line status [job-id]
  line continue --answer <text> [job-id]
  line next [job-id]
  line serve [--listen 127.0.0.1:7340] [--ntfy-topic T]
  line project [add|list|use|current|remove]
  line tui [--db <path>]
  line help

ntfy: optional. Empty --ntfy-topic means no push. Click opens the GitHub issue.
UI: line serve is loopback only. Do not expose it.
`

type CLI struct {
	Stdout     io.Writer
	Stderr     io.Writer
	OpenStore  func(string) (*Store, error)
	NewExec    func(name string, args []string) Executor
	Serve      func(ctx context.Context, addr string, engine *Engine) error
	RunTUI     func(ctx context.Context, engine *Engine) error
	DefaultDB  string
	PromptsDir string
	NtfyTopic  string
	NtfyServer string
	Listen     string
}

func DefaultCLI() CLI {
	home, _ := os.UserHomeDir()
	return CLI{
		Stdout:    os.Stdout,
		Stderr:    os.Stderr,
		OpenStore: OpenStore,
		NewExec: func(name string, args []string) Executor {
			return CommandExecutor{Name: name, Args: args}
		},
		Serve: func(ctx context.Context, addr string, engine *Engine) error {
			return ListenAndServe(ctx, addr, NewUI(engine))
		},
		DefaultDB:  filepath.Join(home, ".line", "line.db"),
		NtfyTopic:  os.Getenv("LINE_NTFY_TOPIC"),
		NtfyServer: ntfyServerFromEnv(),
		Listen:     defaultListen,
	}
}

func (c CLI) Run(ctx context.Context, args []string) int {
	if len(args) == 0 {
		fmt.Fprint(c.Stderr, usageText)
		return 2
	}
	switch args[0] {
	case "help", "--help", "-h":
		fmt.Fprint(c.Stdout, usageText)
		return 0
	case "run":
		return c.cmdRun(ctx, args[1:])
	case "status":
		return c.cmdStatus(ctx, args[1:])
	case "continue":
		return c.cmdContinue(ctx, args[1:])
	case "next":
		return c.cmdNext(ctx, args[1:])
	case "serve":
		return c.cmdServe(ctx, args[1:])
	case "project":
		return c.cmdProject(ctx, args[1:])
	case "tui":
		return c.cmdTUI(ctx, args[1:])
	default:
		fmt.Fprintf(c.Stderr, "line: unknown command %q\n", args[0])
		fmt.Fprint(c.Stderr, usageText)
		return 2
	}
}

func (c CLI) cmdRun(ctx context.Context, args []string) int {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(c.Stderr)
	repo := fs.String("repo", "", "absolute git repository path")
	dbPath := fs.String("db", c.DefaultDB, "sqlite path")
	prompts := fs.String("prompts", c.PromptsDir, "optional directory of phase markdown prompts")
	execName := fs.String("exec", "claude", "agent executable")
	auto := fs.Bool("auto", true, "auto-advance through phases without manual next")
	worktree := fs.Bool("worktree", true, "isolate mutating phases in git worktree")
	reviewExecName := fs.String("review-exec", "", "independent review executable")
	topic, server := c.ntfyFlags(fs)
	var execArgs repeatable
	fs.Var(&execArgs, "exec-arg", "argument passed to --exec (repeatable)")
	var reviewExecArgs repeatable
	fs.Var(&reviewExecArgs, "review-exec-arg", "argument passed to --review-exec (repeatable)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(c.Stderr, "line run: requires one GitHub issue URL")
		return 2
	}
	execPassed := false
	reviewExecPassed := false
	topicPassed := false
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "exec":
			execPassed = true
		case "review-exec":
			reviewExecPassed = true
		case "ntfy-topic":
			topicPassed = true
		}
	})
	if strings.TrimSpace(*repo) == "" {
		if store, err := c.OpenStore(*dbPath); err == nil {
			if active, err := store.GetActiveProject(ctx); err == nil && active.RepoPath != "" {
				*repo = active.RepoPath
				if !execPassed && active.DefaultExec != "" {
					*execName = active.DefaultExec
				}
				if !reviewExecPassed && active.ReviewExec != "" {
					*reviewExecName = active.ReviewExec
				}
				if !topicPassed && active.NtfyTopic != "" {
					*topic = active.NtfyTopic
				}
			}
			_ = store.Close()
		}
	}
	if strings.TrimSpace(*repo) == "" {
		fmt.Fprintln(c.Stderr, "line run: --repo is required or an active project must be set")
		return 2
	}
	if len(execArgs) == 0 {
		execArgs = []string{"--print"}
	}
	engine, closer, err := c.engine(*dbPath, *prompts, *execName, execArgs, *topic, *server)
	if err != nil {
		fmt.Fprintln(c.Stderr, err)
		return 1
	}
	defer closer()
	if *worktree {
		engine.WithWorktreeManager(GitWorktreeManager{})
	}
	if *reviewExecName != "" {
		engine.WithReviewExecutor(c.NewExec(*reviewExecName, reviewExecArgs))
	}
	job, err := engine.RunPlan(ctx, fs.Arg(0), *repo)
	if err != nil {
		fmt.Fprintf(c.Stderr, "line: %v\n", err)
		return 1
	}
	if *auto && job.Status == StatusAwaitingNext {
		job, err = engine.AutoAdvance(ctx, job.ID)
		if err != nil {
			fmt.Fprintf(c.Stderr, "line: %v\n", err)
			return 1
		}
	}
	printJob(c.Stdout, job)
	return exitFor(job)
}

func (c CLI) cmdStatus(ctx context.Context, args []string) int {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	fs.SetOutput(c.Stderr)
	dbPath := fs.String("db", c.DefaultDB, "sqlite path")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	engine, closer, err := c.engine(*dbPath, c.PromptsDir, "claude", nil, "", "")
	if err != nil {
		fmt.Fprintln(c.Stderr, err)
		return 1
	}
	defer closer()
	id := ""
	if fs.NArg() == 1 {
		id = fs.Arg(0)
	}
	job, events, err := engine.Status(ctx, id)
	if err != nil {
		fmt.Fprintf(c.Stderr, "line: %v\n", err)
		return 1
	}
	printJob(c.Stdout, job)
	for _, event := range events {
		fmt.Fprintf(c.Stdout, "event %s %s %s\n", event.Phase, event.Kind, event.Detail)
	}
	return 0
}

func (c CLI) cmdContinue(ctx context.Context, args []string) int {
	fs := flag.NewFlagSet("continue", flag.ContinueOnError)
	fs.SetOutput(c.Stderr)
	dbPath := fs.String("db", c.DefaultDB, "sqlite path")
	answer := fs.String("answer", "", "human answer for needs_human")
	auto := fs.Bool("auto", false, "auto-advance after continue")
	topic, server := c.ntfyFlags(fs)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	engine, closer, err := c.engine(*dbPath, c.PromptsDir, "claude", []string{"--print"}, *topic, *server)
	if err != nil {
		fmt.Fprintln(c.Stderr, err)
		return 1
	}
	defer closer()
	id, err := c.jobID(ctx, engine, fs)
	if err != nil {
		fmt.Fprintf(c.Stderr, "line: %v\n", err)
		return 1
	}
	job, err := engine.Continue(ctx, id, *answer)
	if err != nil {
		fmt.Fprintf(c.Stderr, "line: %v\n", err)
		return 1
	}
	if *auto && job.Status == StatusAwaitingNext {
		job, err = engine.AutoAdvance(ctx, job.ID)
		if err != nil {
			fmt.Fprintf(c.Stderr, "line: %v\n", err)
			return 1
		}
	}
	printJob(c.Stdout, job)
	return exitFor(job)
}

func (c CLI) cmdNext(ctx context.Context, args []string) int {
	fs := flag.NewFlagSet("next", flag.ContinueOnError)
	fs.SetOutput(c.Stderr)
	dbPath := fs.String("db", c.DefaultDB, "sqlite path")
	topic, server := c.ntfyFlags(fs)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	engine, closer, err := c.engine(*dbPath, c.PromptsDir, "claude", []string{"--print"}, *topic, *server)
	if err != nil {
		fmt.Fprintln(c.Stderr, err)
		return 1
	}
	defer closer()
	id, err := c.jobID(ctx, engine, fs)
	if err != nil {
		fmt.Fprintf(c.Stderr, "line: %v\n", err)
		return 1
	}
	job, err := engine.Advance(ctx, id)
	if err != nil {
		fmt.Fprintf(c.Stderr, "line: %v\n", err)
		return 1
	}
	printJob(c.Stdout, job)
	return exitFor(job)
}

func (c CLI) cmdServe(ctx context.Context, args []string) int {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(c.Stderr)
	dbPath := fs.String("db", c.DefaultDB, "sqlite path")
	listenDefault := c.Listen
	if listenDefault == "" {
		listenDefault = defaultListen
	}
	listen := fs.String("listen", listenDefault, "loopback listen address")
	topic, server := c.ntfyFlags(fs)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	engine, closer, err := c.engine(*dbPath, c.PromptsDir, "claude", []string{"--print"}, *topic, *server)
	if err != nil {
		fmt.Fprintln(c.Stderr, err)
		return 1
	}
	defer closer()
	if c.Serve == nil {
		fmt.Fprintln(c.Stderr, "line serve: handler missing")
		return 1
	}
	fmt.Fprintf(c.Stderr, "line: ui http://%s\n", *listen)
	if err := c.Serve(ctx, *listen, engine); err != nil && ctx.Err() == nil {
		fmt.Fprintf(c.Stderr, "line: %v\n", err)
		return 1
	}
	return 0
}

func (c CLI) jobID(ctx context.Context, engine *Engine, fs *flag.FlagSet) (string, error) {
	if fs.NArg() == 1 {
		return fs.Arg(0), nil
	}
	job, _, err := engine.Status(ctx, "")
	if err != nil {
		return "", err
	}
	return job.ID, nil
}

func ntfyServerFromEnv() string {
	if v := strings.TrimSpace(os.Getenv("LINE_NTFY_SERVER")); v != "" {
		return v
	}
	return "https://ntfy.sh"
}

func (c CLI) ntfyFlags(fs *flag.FlagSet) (*string, *string) {
	topic := fs.String("ntfy-topic", c.NtfyTopic, "ntfy topic; empty disables push")
	server := fs.String("ntfy-server", c.NtfyServer, "ntfy server URL")
	return topic, server
}

func (c CLI) engine(dbPath, promptsDir, execName string, execArgs []string, topic, server string) (*Engine, func(), error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o700); err != nil {
		return nil, nil, fmt.Errorf("create db dir: %w", err)
	}
	store, err := c.OpenStore(dbPath)
	if err != nil {
		return nil, nil, err
	}
	exec := c.NewExec(execName, execArgs)
	engine := NewEngine(store, exec, promptsDir).WithNotifier(HTTPNotifier{
		Server: server,
		Topic:  topic,
	})
	return engine, func() { _ = store.Close() }, nil
}

func printJob(w io.Writer, job Job) {
	fmt.Fprintf(w, "job %s\n", job.ID)
	fmt.Fprintf(w, "issue %s\n", job.IssueURL)
	fmt.Fprintf(w, "phase %s\n", job.Phase)
	fmt.Fprintf(w, "status %s\n", job.Status)
	if job.Summary != "" {
		fmt.Fprintf(w, "summary %s\n", job.Summary)
	}
	if job.Question != "" {
		fmt.Fprintf(w, "question %s\n", job.Question)
	}
}

func exitFor(job Job) int {
	switch job.Status {
	case StatusFailed:
		return 1
	default:
		return 0
	}
}

type repeatable []string

func (r *repeatable) String() string { return strings.Join(*r, ",") }
func (r *repeatable) Set(value string) error {
	*r = append(*r, value)
	return nil
}

func (c CLI) store(dbPath string) (*Store, func(), error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o700); err != nil {
		return nil, nil, fmt.Errorf("create db dir: %w", err)
	}
	store, err := c.OpenStore(dbPath)
	if err != nil {
		return nil, nil, err
	}
	return store, func() { _ = store.Close() }, nil
}

func reorderFlags(args []string) []string {
	var flags []string
	var pos []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "-") {
			flags = append(flags, arg)
			if !strings.Contains(arg, "=") && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				flags = append(flags, args[i+1])
				i++
			}
		} else {
			pos = append(pos, arg)
		}
	}
	return append(flags, pos...)
}

func (c CLI) cmdProject(ctx context.Context, args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(c.Stderr, "Usage: line project [add|list|use|current|remove]")
		return 2
	}
	subArgs := reorderFlags(args[1:])
	switch args[0] {
	case "add":
		fs := flag.NewFlagSet("project add", flag.ContinueOnError)
		fs.SetOutput(c.Stderr)
		name := fs.String("name", "", "project display name")
		repo := fs.String("repo", "", "absolute git repository path (required)")
		exec := fs.String("exec", "", "default agent executable")
		reviewExec := fs.String("review-exec", "", "independent review executable")
		ntfyTopic := fs.String("ntfy-topic", "", "ntfy notification topic")
		dbPath := fs.String("db", c.DefaultDB, "sqlite path")
		if err := fs.Parse(subArgs); err != nil {
			return 2
		}
		if fs.NArg() != 1 || strings.TrimSpace(*repo) == "" {
			fmt.Fprintln(c.Stderr, "line project add: requires project id and --repo path")
			return 2
		}
		id := fs.Arg(0)
		projName := *name
		if strings.TrimSpace(projName) == "" {
			projName = id
		}
		store, closer, err := c.store(*dbPath)
		if err != nil {
			fmt.Fprintln(c.Stderr, err)
			return 1
		}
		defer closer()
		p := Project{
			ID:          id,
			Name:        projName,
			RepoPath:    *repo,
			DefaultExec: *exec,
			ReviewExec:  *reviewExec,
			NtfyTopic:   *ntfyTopic,
		}
		if err := store.CreateProject(ctx, p); err != nil {
			fmt.Fprintf(c.Stderr, "line: %v\n", err)
			return 1
		}
		active, _ := store.GetActiveProject(ctx)
		if active.ID == id {
			fmt.Fprintf(c.Stdout, "project %s added (active)\n", id)
		} else {
			fmt.Fprintf(c.Stdout, "project %s added\n", id)
		}
		return 0

	case "list":
		fs := flag.NewFlagSet("project list", flag.ContinueOnError)
		fs.SetOutput(c.Stderr)
		dbPath := fs.String("db", c.DefaultDB, "sqlite path")
		if err := fs.Parse(subArgs); err != nil {
			return 2
		}
		store, closer, err := c.store(*dbPath)
		if err != nil {
			fmt.Fprintln(c.Stderr, err)
			return 1
		}
		defer closer()
		projects, err := store.ListProjects(ctx)
		if err != nil {
			fmt.Fprintf(c.Stderr, "line: %v\n", err)
			return 1
		}
		if len(projects) == 0 {
			fmt.Fprintln(c.Stdout, "no projects registered")
			return 0
		}
		for _, p := range projects {
			mark := " "
			if p.IsActive {
				mark = "*"
			}
			fmt.Fprintf(c.Stdout, "%s %s (%s) - %s\n", mark, p.ID, p.Name, p.RepoPath)
		}
		return 0

	case "use":
		fs := flag.NewFlagSet("project use", flag.ContinueOnError)
		fs.SetOutput(c.Stderr)
		dbPath := fs.String("db", c.DefaultDB, "sqlite path")
		if err := fs.Parse(subArgs); err != nil {
			return 2
		}
		if fs.NArg() != 1 {
			fmt.Fprintln(c.Stderr, "line project use: requires project id")
			return 2
		}
		id := fs.Arg(0)
		store, closer, err := c.store(*dbPath)
		if err != nil {
			fmt.Fprintln(c.Stderr, err)
			return 1
		}
		defer closer()
		if err := store.SetActiveProject(ctx, id); err != nil {
			fmt.Fprintf(c.Stderr, "line: %v\n", err)
			return 1
		}
		fmt.Fprintf(c.Stdout, "switched to project %s\n", id)
		return 0

	case "current":
		fs := flag.NewFlagSet("project current", flag.ContinueOnError)
		fs.SetOutput(c.Stderr)
		dbPath := fs.String("db", c.DefaultDB, "sqlite path")
		if err := fs.Parse(subArgs); err != nil {
			return 2
		}
		store, closer, err := c.store(*dbPath)
		if err != nil {
			fmt.Fprintln(c.Stderr, err)
			return 1
		}
		defer closer()
		p, err := store.GetActiveProject(ctx)
		if err != nil || p.ID == "" {
			fmt.Fprintln(c.Stdout, "no active project")
			return 0
		}
		fmt.Fprintf(c.Stdout, "active project: %s (%s) - %s\n", p.ID, p.Name, p.RepoPath)
		return 0

	case "remove":
		fs := flag.NewFlagSet("project remove", flag.ContinueOnError)
		fs.SetOutput(c.Stderr)
		dbPath := fs.String("db", c.DefaultDB, "sqlite path")
		if err := fs.Parse(subArgs); err != nil {
			return 2
		}
		if fs.NArg() != 1 {
			fmt.Fprintln(c.Stderr, "line project remove: requires project id")
			return 2
		}
		id := fs.Arg(0)
		store, closer, err := c.store(*dbPath)
		if err != nil {
			fmt.Fprintln(c.Stderr, err)
			return 1
		}
		defer closer()
		if err := store.DeleteProject(ctx, id); err != nil {
			fmt.Fprintf(c.Stderr, "line: %v\n", err)
			return 1
		}
		fmt.Fprintf(c.Stdout, "removed project %s\n", id)
		return 0

	default:
		fmt.Fprintf(c.Stderr, "line project: unknown subcommand %q\n", args[0])
		fmt.Fprintln(c.Stderr, "Usage: line project [add|list|use|current|remove]")
		return 2
	}
}

func (c CLI) cmdTUI(ctx context.Context, args []string) int {
	fs := flag.NewFlagSet("tui", flag.ContinueOnError)
	fs.SetOutput(c.Stderr)
	dbPath := fs.String("db", c.DefaultDB, "sqlite path")
	prompts := fs.String("prompts", c.PromptsDir, "optional directory of phase markdown prompts")
	execName := fs.String("exec", "claude", "agent executable")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if c.RunTUI == nil {
		fmt.Fprintln(c.Stderr, "line tui: interactive terminal interface not available")
		return 1
	}
	engine, closer, err := c.engine(*dbPath, *prompts, *execName, []string{"--print"}, "", "")
	if err != nil {
		fmt.Fprintln(c.Stderr, err)
		return 1
	}
	defer closer()
	if err := c.RunTUI(ctx, engine); err != nil {
		fmt.Fprintf(c.Stderr, "line tui: %v\n", err)
		return 1
	}
	return 0
}
