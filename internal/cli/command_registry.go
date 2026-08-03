package cli

import (
	"fmt"
	"sort"
	"strings"
	"unicode"
)

// CommandMeta defines canonical user-facing metadata for a top-level CLI command.
type CommandMeta struct {
	ID          string
	DisplayName string
	Description string
	Aliases     []string
	Group       string
	Usage       string
	Examples    []string
	Intent      []string
	Related     []string
	Notes       []string
	Suggested   string
	ReadOnly    bool
	SafeFuzzy   bool
}

type HelpTopic struct {
	Aliases   []string
	CommandID string
}

var topLevelCommands = []CommandMeta{
	{ID: "init", DisplayName: "init", Description: "Set up the local vault and database", Aliases: []string{"setup", "setup vault", "set up vault"}, Group: "Setup", Usage: "skillvault setup [--with-secrets] [--with-telemetry] [--all]", Examples: []string{"skillvault setup", "skillvault init --all"}, Intent: []string{"initialize vault", "create local vault", "first-time setup"}, Related: []string{"doctor", "mcp-config"}, Notes: []string{"Creates ~/.skillvault, required subdirectories, and the SQLite database.", "Safe to rerun if the vault already exists."}, Suggested: "setup"},
	{ID: "doctor", DisplayName: "doctor", Description: "Check whether the vault is ready to use", Aliases: []string{"check", "setup doctor", "doctor setup", "check setup", "check vault"}, Group: "Setup", Usage: "skillvault doctor", Examples: []string{"skillvault doctor", "skillvault check", "skillvault setup doctor"}, Intent: []string{"check setup", "is the vault ready", "diagnose vault"}, Related: []string{"init", "backup"}, Notes: []string{"Read-only: this command does not initialize, migrate, or repair the vault.", "Use it before asking an agent to rely on local memory."}, Suggested: "doctor", ReadOnly: true, SafeFuzzy: true},
	{ID: "version", DisplayName: "version", Description: "Print version information", Group: "Setup"},
	{ID: "update", DisplayName: "update", Description: "Rebuild and reinstall the skillvault binary from source", Group: "Setup"},
	{ID: "install-telemetry", DisplayName: "install-telemetry", Description: "Build and install telemetryd, telemetryctl, and telemetrywrap from the kbs tool suite repo", Group: "Setup"},
	{ID: "add-entry", DisplayName: "add-entry", Description: "Save a new entry to the vault", Group: "Store"},
	{ID: "save-artifact", DisplayName: "save-artifact", Description: "Save a file-backed artifact to the vault", Group: "Store"},
	{ID: "save-result", DisplayName: "save-result", Description: "Save an AI prompt result to the vault", Group: "Store"},
	{ID: "archive", DisplayName: "archive", Description: "Archive an entry without deleting it", Group: "Store"},
	{ID: "search", DisplayName: "search", Description: "Find entries with full-text or vector search", Aliases: []string{"find", "lookup", "look up", "search memory"}, Group: "Find", Usage: "skillvault find \"query\" [--project myapp] [--type skill] [--limit 10]", Examples: []string{"skillvault find \"auth middleware\"", "skillvault search \"release notes\" --project codex --limit 5", "skillvault search \"semantic memory\" --vector"}, Intent: []string{"search memory", "find saved entry", "look up notes"}, Related: []string{"get", "get-context", "backup"}, Notes: []string{"The first positional value is the query string.", "Use --vector only after configuring embeddings with setup-vectors."}, Suggested: "find", ReadOnly: true, SafeFuzzy: true},
	{ID: "get", DisplayName: "get", Description: "Read an entry by ID or slug", Aliases: []string{"read", "open", "open entry", "show entry"}, Group: "Find", Usage: "skillvault read <entry-id-or-slug>", Examples: []string{"skillvault read auth-middleware-review", "skillvault open auth-middleware-review", "skillvault get skill:auth-middleware-review"}, Intent: []string{"open saved entry", "read note", "show entry details"}, Related: []string{"search", "get-context"}, Notes: []string{"Use search first if you do not remember the exact ID or slug."}, Suggested: "read", ReadOnly: true, SafeFuzzy: true},
	{ID: "get-context", DisplayName: "get-context", Description: "Build a compact context pack for an agent", Aliases: []string{"context", "context project", "project context"}, Group: "Find", Usage: "skillvault context --project <project> [--mode planning] [--query \"topic\"]", Examples: []string{"skillvault context --project codex --mode planning", "skillvault context project --project codex", "skillvault get-context --project codex --query \"auth rollout\" --max-chars 8000"}, Intent: []string{"prepare agent context", "build brief", "get project context"}, Related: []string{"search", "get", "session-wrap"}, Notes: []string{"Read-only: it prints a context pack and does not modify stored entries."}, Suggested: "context", ReadOnly: true, SafeFuzzy: true},
	{ID: "graph", DisplayName: "graph", Description: "Traverse and render the entry reference graph", Group: "Find"},
	{ID: "compare-entries", DisplayName: "compare-entries", Description: "Show unified diff between two entries", Group: "Find"},
	{ID: "add-project", DisplayName: "add-project", Description: "Create a new project in the vault", Aliases: []string{"project add", "project start"}, Group: "Context", Usage: "skillvault project start --name \"MyApp\" [--description \"What it is\"]", Examples: []string{"skillvault project start --name \"MyApp\"", "skillvault add-project --name \"Codex\" --description \"CLI intelligence work\""}, Intent: []string{"start project", "create project", "new project"}, Related: []string{"list-projects", "get-context", "session-wrap"}, Notes: []string{"This is an additive shortcut over the existing add-project command."}, Suggested: "project start"},
	{ID: "list-projects", DisplayName: "list-projects", Description: "List projects in the vault", Aliases: []string{"projects", "project list", "show projects"}, Group: "Context", Intent: []string{"show projects", "browse projects"}, Suggested: "projects", ReadOnly: true, SafeFuzzy: true},
	{ID: "pending", DisplayName: "pending", Description: "Capture, review, and resolve per-project pending items", Aliases: []string{"todo"}, Group: "Context", Usage: "skillvault pending add --project myapp \"Update presentation\"", Examples: []string{"skillvault pending add --project myapp \"Update presentation\"", "skillvault pending list --project myapp --query review", "skillvault pending review --project myapp", "skillvault pending show update-presentation", "skillvault pending done update-presentation"}, Intent: []string{"save deferred work", "review pending items", "show todo details", "mark todo done"}, Related: []string{"add-project", "search", "archive"}, Notes: []string{"Pending items are stored as normal entries with type `pending` and purpose `WORK`.", "Use `pending review` for a project-scoped review pass and `pending show <id>` for a single item.", "Active project pending items appear in the default context pack and can be requested explicitly with `--include pending`."}, Suggested: "pending"},
	{ID: "session-wrap", DisplayName: "session-wrap", Description: "Save a session summary with decisions and pending items", Aliases: []string{"session"}, Group: "Context"},
	{ID: "add-workflow", DisplayName: "add-workflow", Description: "Create a workflow from a JSON definition file", Group: "Workflows"},
	{ID: "import-workflow", DisplayName: "import-workflow", Description: "Import a workflow-builder YAML file as entries plus workflow", Aliases: []string{"workflow import"}, Group: "Workflows"},
	{ID: "render-workflow", DisplayName: "render-workflow", Description: "Render a workflow as a human-readable checklist", Aliases: []string{"workflow show"}, Group: "Workflows"},
	{ID: "route", DisplayName: "route", Description: "Resolve a scenario to its matching workflow or skill", Group: "Workflows"},
	{ID: "run", DisplayName: "run", Description: "Execute a workflow pipeline with input", Group: "Workflows"},
	{ID: "backup", DisplayName: "backup", Description: "Write a dated JSON backup of the vault", Aliases: []string{"backup all"}, Group: "Maintenance", Usage: "skillvault backup", Examples: []string{"skillvault backup", "skillvault backup all"}, Intent: []string{"backup everything", "snapshot vault", "export a dated backup"}, Related: []string{"doctor", "export", "import"}, Notes: []string{"Writes a timestamped JSON snapshot into ~/.skillvault/exports/."}, Suggested: "backup"},
	{ID: "export", DisplayName: "export", Description: "Export vault contents to a JSON file", Group: "Maintenance"},
	{ID: "import", DisplayName: "import", Description: "Import vault contents from a JSON file", Group: "Maintenance"},
	{ID: "stats", DisplayName: "stats", Description: "Show vault statistics and entry counts", Group: "Maintenance"},
	{ID: "sync-pull", DisplayName: "sync pull", Description: "Pull a vault snapshot from remote storage", Aliases: []string{"sync-pull"}, Group: "Maintenance"},
	{ID: "sync-push", DisplayName: "sync push", Description: "Push a vault snapshot to remote storage", Aliases: []string{"sync-push"}, Group: "Maintenance"},
	{ID: "entry-history", DisplayName: "entry history", Description: "Show version history for an entry", Aliases: []string{"entry-history"}, Group: "Maintenance"},
	{ID: "entry-restore", DisplayName: "entry restore", Description: "Restore an entry to a previous version", Aliases: []string{"entry-restore"}, Group: "Maintenance"},
	{ID: "entry-ref", DisplayName: "entry ref", Description: "Manage entry reference links (add, list, remove)", Aliases: []string{"ref", "entry-ref"}, Group: "Maintenance"},
	{ID: "memory-index", DisplayName: "memory index", Description: "Index pi-memory markdown files into the vault", Aliases: []string{"memory-index"}, Group: "Maintenance"},
	{ID: "memory-reindex", DisplayName: "memory reindex", Description: "Reindex all memory entries from external sources", Aliases: []string{"memory-reindex"}, Group: "Maintenance"},
	{ID: "memory-list-external", DisplayName: "memory list-external", Description: "List shadow entries linked to external memory files", Aliases: []string{"memory-list-external"}, Group: "Maintenance"},
	{ID: "setup-vectors", DisplayName: "setup-vectors", Description: "Load GloVe word vectors for semantic search", Group: "Maintenance"},
	{ID: "reindex-embeddings", DisplayName: "reindex-embeddings", Description: "Recompute vector embeddings for all vault entries", Group: "Maintenance"},
	{ID: "mcp", DisplayName: "mcp", Description: "Start the MCP JSON-RPC 2.0 server over stdio", Group: "Integrations"},
	{ID: "mcp-config", DisplayName: "mcp config", Description: "Print a ready-to-paste MCP client config snippet", Aliases: []string{"mcp-config", "setup mcp", "mcp setup"}, Group: "Integrations", Usage: "skillvault mcp config", Examples: []string{"skillvault mcp config", "skillvault setup mcp"}, Intent: []string{"configure mcp", "wire opencode", "show mcp snippet"}, Related: []string{"init", "doctor", "mcp"}, Notes: []string{"Read-only: it prints JSON to stdout and does not touch opencode.json or Claude config files."}, Suggested: "mcp config", ReadOnly: true, SafeFuzzy: true},
	{ID: "http", DisplayName: "http", Description: "Start HTTP REST API server on 127.0.0.1:7438", Group: "Integrations"},
	{ID: "tui", DisplayName: "tui", Description: "Start the Bubble Tea terminal UI for project overview and pending review", Group: "Integrations", Usage: "skillvault tui", Examples: []string{"make build-tui && ./skillvault-tui tui", "go build -tags tui -o skillvault-tui ./cmd/skillvault && ./skillvault-tui tui"}, Intent: []string{"browse projects interactively", "review pending work in tui", "open terminal ui"}, Related: []string{"list-projects", "pending", "get-context", "search"}, Notes: []string{"The TUI stays intentionally light: it shows projects, pending items, entry browsing, and a compact context preview.", "From the pending pane you can mark one item done with confirmation; this reuses the same archive semantics as `skillvault pending done`.", "Default builds keep the TUI disabled behind the `tui` build tag. Without that tag, `skillvault tui` prints the rebuild hint and exits."}, SafeFuzzy: true},
	{ID: "secrets", DisplayName: "secrets", Description: "Run q-secrets or install it via 'secrets install'", Aliases: []string{"q-secrets"}, Group: "Integrations"},
}

var helpTopics = []HelpTopic{
	{Aliases: []string{"help", "docs", "readme", "commands", "open docs", "open help"}},
	{Aliases: []string{"doctor help", "help doctor"}, CommandID: "doctor"},
	{Aliases: []string{"context help", "help context"}, CommandID: "get-context"},
	{Aliases: []string{"mcp help", "help mcp", "mcp config help"}, CommandID: "mcp-config"},
}

var topLevelCommandIndex = buildTopLevelCommandIndex(topLevelCommands)
var topLevelAliasIndex = buildTopLevelAliasIndex(topLevelCommands)
var helpTopicIndex = buildHelpTopicIndex(helpTopics)

func buildTopLevelCommandIndex(commands []CommandMeta) map[string]CommandMeta {
	index := make(map[string]CommandMeta, len(commands))
	for _, command := range commands {
		index[command.ID] = command
	}
	return index
}

func buildTopLevelAliasIndex(commands []CommandMeta) map[string]string {
	index := make(map[string]string)
	for _, command := range commands {
		for _, alias := range append([]string{command.ID, command.DisplayName}, command.Aliases...) {
			key := normalizeCommandKey(alias)
			if key == "" {
				continue
			}
			index[key] = command.ID
		}
	}
	return index
}

func buildHelpTopicIndex(topics []HelpTopic) map[string]HelpTopic {
	index := make(map[string]HelpTopic)
	for _, topic := range topics {
		for _, alias := range topic.Aliases {
			key := normalizeCommandKey(alias)
			if key == "" {
				continue
			}
			index[key] = topic
		}
	}
	return index
}

func normalizeCommandKey(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	normalized = strings.Map(func(r rune) rune {
		switch {
		case unicode.IsLetter(r), unicode.IsNumber(r):
			return r
		case r == '-', r == '_', unicode.IsSpace(r):
			return ' '
		default:
			return -1
		}
	}, normalized)
	return strings.Join(strings.Fields(normalized), " ")
}

// TopLevelCommands returns the canonical metadata for top-level CLI commands.
func TopLevelCommands() []CommandMeta {
	commands := make([]CommandMeta, len(topLevelCommands))
	copy(commands, topLevelCommands)
	return commands
}

// TopLevelCommandMeta returns metadata for a canonical command ID.
func TopLevelCommandMeta(id string) (CommandMeta, bool) {
	command, ok := topLevelCommandIndex[id]
	return command, ok
}

// ResolveTopLevelCommand resolves one-word and nested aliases to a canonical command ID.
func ResolveTopLevelCommand(parts []string) (string, int, bool) {
	maxParts := 3
	if len(parts) < maxParts {
		maxParts = len(parts)
	}
	for width := maxParts; width >= 1; width-- {
		key := normalizeCommandKey(strings.Join(parts[:width], " "))
		if id, ok := topLevelAliasIndex[key]; ok {
			return id, width, true
		}
	}
	return "", 0, false
}

// ResolveHelpTopic resolves side-effect-free help and docs shortcuts.
func ResolveHelpTopic(parts []string) (string, bool, bool) {
	maxParts := 3
	if len(parts) < maxParts {
		maxParts = len(parts)
	}
	for width := maxParts; width >= 1; width-- {
		key := normalizeCommandKey(strings.Join(parts[:width], " "))
		if topic, ok := helpTopicIndex[key]; ok {
			return topic.CommandID, topic.CommandID != "", true
		}
	}
	return "", false, false
}

func fuzzyTopLevelCommand(part string) (string, bool) {
	query := normalizeCommandKey(part)
	if query == "" || len(query) < 4 {
		return "", false
	}

	var best CommandMeta
	bestScore := 0
	for _, command := range topLevelCommands {
		if !command.SafeFuzzy {
			continue
		}
		score := bestDirectAliasMatch(query, command)
		if score > bestScore {
			bestScore = score
			best = command
		}
	}
	if bestScore < 35 {
		return "", false
	}
	return best.ID, true
}

// NormalizeArgs rewrites intent aliases and nested commands to canonical command IDs.
func NormalizeArgs(args []string) []string {
	if len(args) < 2 {
		return args
	}
	id, consumed, ok := ResolveTopLevelCommand(args[1:])
	if !ok {
		if id, ok := fuzzyTopLevelCommand(args[1]); ok {
			normalized := make([]string, 0, len(args))
			normalized = append(normalized, args[0], id)
			normalized = append(normalized, args[2:]...)
			return normalized
		}
		return args
	}
	normalized := make([]string, 0, len(args)-consumed+1)
	normalized = append(normalized, args[0], id)
	normalized = append(normalized, args[1+consumed:]...)
	return normalized
}

// CommandGroups returns top-level commands grouped by user intent.
func CommandGroups() []struct {
	Name     string
	Commands []CommandMeta
} {
	grouped := make(map[string][]CommandMeta)
	order := []string{"Setup", "Store", "Find", "Context", "Workflows", "Maintenance", "Integrations"}
	for _, command := range topLevelCommands {
		grouped[command.Group] = append(grouped[command.Group], command)
	}
	result := make([]struct {
		Name     string
		Commands []CommandMeta
	}, 0, len(order))
	for _, name := range order {
		commands := grouped[name]
		if len(commands) == 0 {
			continue
		}
		sort.Slice(commands, func(i, j int) bool {
			return commands[i].DisplayName < commands[j].DisplayName
		})
		result = append(result, struct {
			Name     string
			Commands []CommandMeta
		}{Name: name, Commands: commands})
	}
	return result
}

// TopLevelCommandDescription returns the user-facing description for a command.
func TopLevelCommandDescription(id string) string {
	if command, ok := TopLevelCommandMeta(id); ok {
		return command.Description
	}
	return fmt.Sprintf("Execute %s command", id)
}

// PreferredInvocation returns the most natural user-facing invocation for a command.
func PreferredInvocation(command CommandMeta) string {
	if command.Suggested != "" {
		return command.Suggested
	}
	return command.DisplayName
}

// SuggestTopLevelCommands returns likely intent matches for a command-like input.
func SuggestTopLevelCommands(input string, limit int) []CommandMeta {
	query := normalizeCommandKey(strings.ReplaceAll(input, "-", " "))
	if query == "" || limit <= 0 {
		return nil
	}

	type scored struct {
		command CommandMeta
		score   int
	}

	scoredCommands := make([]scored, 0, len(topLevelCommands))
	for _, command := range topLevelCommands {
		best, _ := bestCommandMatch(query, command)
		if best > 0 {
			scoredCommands = append(scoredCommands, scored{command: command, score: best})
		}
	}

	sort.Slice(scoredCommands, func(i, j int) bool {
		if scoredCommands[i].score == scoredCommands[j].score {
			return scoredCommands[i].command.DisplayName < scoredCommands[j].command.DisplayName
		}
		return scoredCommands[i].score > scoredCommands[j].score
	})

	results := make([]CommandMeta, 0, limit)
	seen := make(map[string]struct{}, limit)
	for _, item := range scoredCommands {
		if _, ok := seen[item.command.ID]; ok {
			continue
		}
		seen[item.command.ID] = struct{}{}
		results = append(results, item.command)
		if len(results) == limit {
			break
		}
	}
	return results
}

// UnknownCommandMessage formats a more helpful error for mistyped or vague commands.
func UnknownCommandMessage(input string) string {
	message := fmt.Sprintf("unknown command: %s", input)
	if target, showCommand, ok := ResolveHelpTopic(strings.Fields(input)); ok {
		if !showCommand {
			return message + "\nRun `skillvault help` for side-effect-free command discovery."
		}
		if command, ok := TopLevelCommandMeta(target); ok {
			return fmt.Sprintf("%s\nTry `skillvault help %s` for side-effect-free examples.", message, PreferredInvocation(command))
		}
	}
	suggestions := SuggestTopLevelCommands(input, 4)
	if len(suggestions) == 0 {
		return message + "\nRun `skillvault help` to browse commands."
	}

	var b strings.Builder
	b.WriteString(message)
	b.WriteString("\n\nTry one of these intent-first commands:\n")
	for _, suggestion := range suggestions {
		matchHint := suggestionMatchHint(input, suggestion)
		fmt.Fprintf(&b, "  skillvault %s  %s", PreferredInvocation(suggestion), suggestion.Description)
		if matchHint != "" {
			fmt.Fprintf(&b, " (matches: %s)", matchHint)
		}
		b.WriteString("\n")
	}
	b.WriteString("Run `skillvault help <command>` for examples.")
	return b.String()
}

func bestCommandMatch(query string, command CommandMeta) (int, string) {
	bestScore := 0
	bestTerm := ""
	for _, term := range commandTerms(command) {
		if score := scoreCommandTerm(query, term); score > bestScore {
			bestScore = score
			bestTerm = normalizeCommandKey(term)
		}
	}
	return bestScore, bestTerm
}

func bestDirectAliasMatch(query string, command CommandMeta) int {
	bestScore := 0
	for _, term := range append([]string{command.ID, command.DisplayName}, command.Aliases...) {
		if score := scoreCommandTerm(query, term); score > bestScore {
			bestScore = score
		}
	}
	return bestScore
}

func suggestionMatchHint(input string, command CommandMeta) string {
	_, term := bestCommandMatch(normalizeCommandKey(input), command)
	if term == "" {
		return ""
	}
	for _, value := range []string{command.ID, command.DisplayName, command.Suggested} {
		if term == normalizeCommandKey(value) {
			return ""
		}
	}
	return term
}

func commandTerms(command CommandMeta) []string {
	terms := make([]string, 0, 2+len(command.Aliases)+len(command.Intent))
	terms = append(terms, command.ID, command.DisplayName)
	terms = append(terms, command.Aliases...)
	terms = append(terms, command.Intent...)
	if command.Suggested != "" {
		terms = append(terms, command.Suggested)
	}
	for _, phrase := range append(append([]string{}, command.Aliases...), command.Intent...) {
		for _, token := range strings.Fields(normalizeCommandKey(strings.ReplaceAll(phrase, "-", " "))) {
			terms = append(terms, token)
		}
	}
	return terms
}

func scoreCommandTerm(query, term string) int {
	normalizedTerm := normalizeCommandKey(strings.ReplaceAll(term, "-", " "))
	if normalizedTerm == "" {
		return 0
	}
	if query == normalizedTerm {
		return 100
	}
	if strings.HasPrefix(normalizedTerm, query) || strings.HasPrefix(query, normalizedTerm) {
		return 85
	}
	if strings.Contains(normalizedTerm, query) || strings.Contains(query, normalizedTerm) {
		return 75
	}
	overlap := tokenOverlapScore(query, normalizedTerm)
	if overlap > 0 {
		return 50 + overlap
	}
	distance := levenshteinDistance(query, normalizedTerm)
	if distance <= 2 {
		return 45 - (distance * 10)
	}
	if distance == 3 && len(query) >= 5 {
		return 10
	}
	return 0
}

func tokenOverlapScore(a, b string) int {
	aTokens := strings.Fields(a)
	bTokens := strings.Fields(b)
	if len(aTokens) == 0 || len(bTokens) == 0 {
		return 0
	}
	bSet := make(map[string]struct{}, len(bTokens))
	for _, token := range bTokens {
		bSet[token] = struct{}{}
	}
	overlap := 0
	for _, token := range aTokens {
		if _, ok := bSet[token]; ok {
			overlap++
		}
	}
	return overlap * 5
}

func levenshteinDistance(a, b string) int {
	if a == b {
		return 0
	}
	if a == "" {
		return len(b)
	}
	if b == "" {
		return len(a)
	}

	prev := make([]int, len(b)+1)
	for j := 0; j <= len(b); j++ {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		current := make([]int, len(b)+1)
		current[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 0
			if a[i-1] != b[j-1] {
				cost = 1
			}
			current[j] = min3(
				current[j-1]+1,
				prev[j]+1,
				prev[j-1]+cost,
			)
		}
		prev = current
	}
	return prev[len(b)]
}

func min3(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}
