package autocomplete

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/lyc-aon/ratatui-go/ompui/editor"
	"github.com/lyc-aon/ratatui-go/ompui/model"
)

// Provider is OMP's CombinedAutocompleteProvider for the Go editor.
//
// Implements:
//   - editor.AutocompleteProvider
//   - editor.InlineHintProvider
//   - editor.SyncSlashProvider
//   - editor.ForceFileProvider
//
// GetSuggestions is safe for concurrent calls. Each call stamps a request ID;
// Cancel drops in-flight work. File I/O uses context cancellation so stale
// results never apply when the caller checks RequestID.
type Provider struct {
	mu       sync.RWMutex
	commands []SlashCommand
	basePath string
	homeDir  string
	files    FileSource
	allowOut bool

	// reqSeq is incremented on every public suggestion call and on Cancel.
	reqSeq atomic.Uint64
	// cancel holds the cancel func for the latest file-backed request.
	cancelMu sync.Mutex
	cancel   context.CancelFunc
}

// New builds a Provider from Options.
func New(opts Options) *Provider {
	base := opts.BasePath
	if base == "" {
		if wd, err := os.Getwd(); err == nil {
			base = wd
		} else {
			base = "."
		}
	}
	base = filepath.Clean(base)

	home := opts.HomeDir
	if home == "" {
		if h, err := os.UserHomeDir(); err == nil {
			home = h
		}
	}

	files := opts.Files
	if files == nil {
		fs := NewFSSource()
		if opts.MaxResults > 0 {
			fs.MaxResults = opts.MaxResults
		}
		if opts.MaxScanEntries > 0 {
			fs.MaxScanEntries = opts.MaxScanEntries
		}
		if opts.MaxDepth > 0 {
			fs.MaxDepth = opts.MaxDepth
		}
		files = fs
	}

	allowOut := true
	if opts.AllowOutsideRoot != nil {
		allowOut = *opts.AllowOutsideRoot
	}

	p := &Provider{
		commands: append([]SlashCommand(nil), opts.Commands...),
		basePath: base,
		homeDir:  home,
		files:    files,
		allowOut: allowOut,
	}
	return p
}

// SetCommands replaces the slash command registry (preserves order).
func (p *Provider) SetCommands(cmds []SlashCommand) {
	p.mu.Lock()
	p.commands = append([]SlashCommand(nil), cmds...)
	p.mu.Unlock()
}

// SetModelCommands maps and installs model.AvailableCommand entries.
func (p *Provider) SetModelCommands(cmds []model.AvailableCommand) {
	p.SetCommands(CommandsFromModel(cmds))
}

// BasePath returns the configured project root.
func (p *Provider) BasePath() string {
	return p.basePath
}

// Cancel aborts in-flight file discovery and invalidates pending request IDs.
func (p *Provider) Cancel() {
	p.reqSeq.Add(1)
	p.cancelMu.Lock()
	if p.cancel != nil {
		p.cancel()
		p.cancel = nil
	}
	p.cancelMu.Unlock()
}

// SnapshotRequestID returns the current generation. Callers comparing against
// a value captured before GetSuggestions can drop stale UI applies.
func (p *Provider) SnapshotRequestID() uint64 {
	return p.reqSeq.Load()
}

func (p *Provider) snapshotCommands() []SlashCommand {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := make([]SlashCommand, len(p.commands))
	copy(out, p.commands)
	return out
}

func (p *Provider) beginRequest() (ctx context.Context, reqID uint64) {
	p.reqSeq.Add(1)
	reqID = p.reqSeq.Load()
	ctx, cancel := context.WithCancel(context.Background())
	p.cancelMu.Lock()
	if p.cancel != nil {
		p.cancel()
	}
	p.cancel = cancel
	p.cancelMu.Unlock()
	return ctx, reqID
}

func (p *Provider) stillCurrent(reqID uint64) bool {
	return p.reqSeq.Load() == reqID
}

// ---------------------------------------------------------------------------
// editor.AutocompleteProvider
// ---------------------------------------------------------------------------

// GetSuggestions implements editor.AutocompleteProvider.
func (p *Provider) GetSuggestions(lines []string, cursorLine, cursorCol int) *editor.Suggestions {
	ctx, reqID := p.beginRequest()

	textBefore, ok := textBeforeCursor(lines, cursorLine, cursorCol)
	if !ok {
		return nil
	}

	// @ file reference (fuzzy) — must be after a delimiter or at start.
	if atPrefix := p.extractAtPrefix(textBefore); atPrefix != "" {
		return p.atSuggestions(ctx, reqID, atPrefix)
	}

	if slashStart := FindLeadingSlashCommandStart(textBefore); slashStart >= 0 {
		return p.slashSuggestions(textBefore, slashStart)
	}

	// Natural path trigger (non-@).
	pathMatch, hasPath := p.extractPathPrefixOK(textBefore, false)
	if hasPath {
		items := p.getFileSuggestions(ctx, pathMatch)
		if !p.stillCurrent(reqID) || len(items) == 0 {
			return nil
		}
		return &editor.Suggestions{Items: items, Prefix: pathMatch}
	}
	return nil
}

// ApplyCompletion implements editor.AutocompleteProvider.
func (p *Provider) ApplyCompletion(lines []string, cursorLine, cursorCol int, item editor.AutocompleteItem, prefix string) editor.CompletionResult {
	out := cloneLines(lines)
	if cursorLine < 0 || cursorLine >= len(out) {
		return editor.CompletionResult{Lines: out, CursorLine: max(0, cursorLine), CursorCol: cursorCol}
	}
	currentLine := out[cursorLine]
	if cursorCol < 0 {
		cursorCol = 0
	}
	if cursorCol > len(currentLine) {
		cursorCol = len(currentLine)
	}
	textBeforeCursor := currentLine[:cursorCol]
	afterCursor := currentLine[cursorCol:]

	slashStart := FindLeadingSlashCommandStart(textBeforeCursor)

	// Live slash-token replace for stale rendered suggestions (OMP Enter race).
	if FindLeadingSlashCommandStart(prefix) >= 0 && slashStart >= 0 {
		slashPrefix := textBeforeCursor[slashStart:]
		if !strings.Contains(slashPrefix, " ") && !strings.Contains(slashPrefix[1:], "/") {
			beforeSlash := currentLine[:slashStart]
			newLine := beforeSlash + "/" + item.Value + " " + afterCursor
			out[cursorLine] = newLine
			return editor.CompletionResult{
				Lines:      out,
				CursorLine: cursorLine,
				CursorCol:  len(beforeSlash) + 1 + len(item.Value) + 1, // "/" + value + " "
			}
		}
	}

	beforePrefix := ""
	if len(prefix) <= cursorCol {
		beforePrefix = currentLine[:cursorCol-len(prefix)]
	} else {
		beforePrefix = currentLine[:cursorCol]
	}

	// @ file attachment: prefer live @ token length over stale prefix.
	if strings.HasPrefix(prefix, "@") {
		if live := p.extractAtPrefix(textBeforeCursor); live != "" {
			beforePrefix = currentLine[:cursorCol-len(live)]
		}
		newLine := beforePrefix + item.Value + " " + afterCursor
		out[cursorLine] = newLine
		return editor.CompletionResult{
			Lines:      out,
			CursorLine: cursorLine,
			CursorCol:  len(beforePrefix) + len(item.Value) + 1,
		}
	}

	// Slash args + plain path: replace rendered prefix.
	newLine := beforePrefix + item.Value + afterCursor
	out[cursorLine] = newLine
	return editor.CompletionResult{
		Lines:      out,
		CursorLine: cursorLine,
		CursorCol:  len(beforePrefix) + len(item.Value),
	}
}

// ---------------------------------------------------------------------------
// Optional hooks
// ---------------------------------------------------------------------------

// GetInlineHint implements editor.InlineHintProvider.
func (p *Provider) GetInlineHint(lines []string, cursorLine, cursorCol int) string {
	textBefore, ok := textBeforeCursor(lines, cursorLine, cursorCol)
	if !ok {
		return ""
	}
	slashStart := FindLeadingSlashCommandStart(textBefore)
	if slashStart < 0 {
		return ""
	}
	commandText := textBefore[slashStart:]
	spaceIndex := strings.IndexByte(commandText, ' ')
	if spaceIndex < 0 {
		return ""
	}
	commandName := commandText[1:spaceIndex]
	argumentText := commandText[spaceIndex+1:]
	cmd := findCommand(p.snapshotCommands(), commandName)
	if cmd == nil || cmd.InlineHint == nil {
		return ""
	}
	return cmd.InlineHint(argumentText)
}

// TrySyncSlashCompletion implements editor.SyncSlashProvider.
// Synchronous exact slash replacement path for Enter-before-debounce races.
func (p *Provider) TrySyncSlashCompletion(textBeforeCursor string) *editor.Suggestions {
	slashStart := FindLeadingSlashCommandStart(textBeforeCursor)
	if slashStart < 0 {
		return nil
	}
	commandText := textBeforeCursor[slashStart:]
	if len(commandText) <= 1 {
		return nil // bare "/"
	}
	if strings.Contains(commandText, " ") {
		return nil
	}
	prefix := commandText[1:]
	lowerPrefix := strings.ToLower(prefix)
	matches := buildSlashCommandCompletions(p.snapshotCommands(), lowerPrefix)
	if len(matches) == 0 {
		return nil
	}
	// Full text-before-cursor as prefix so Enter-staleness check still applies.
	return &editor.Suggestions{Items: matches, Prefix: textBeforeCursor}
}

// GetForceFileSuggestions implements editor.ForceFileProvider.
func (p *Provider) GetForceFileSuggestions(lines []string, cursorLine, cursorCol int) *editor.Suggestions {
	ctx, reqID := p.beginRequest()
	textBefore, ok := textBeforeCursor(lines, cursorLine, cursorCol)
	if !ok {
		return nil
	}
	// Don't force-file while typing a slash command name at line start.
	trim := strings.TrimSpace(textBefore)
	if strings.HasPrefix(trim, "/") && !strings.Contains(trim, " ") {
		return nil
	}
	pathMatch, has := p.extractPathPrefixOK(textBefore, true)
	if !has {
		return nil
	}
	items := p.getFileSuggestions(ctx, pathMatch)
	if !p.stillCurrent(reqID) || len(items) == 0 {
		return nil
	}
	return &editor.Suggestions{Items: items, Prefix: pathMatch}
}

// ShouldTriggerFileCompletion implements editor.ForceFileProvider.
func (p *Provider) ShouldTriggerFileCompletion(lines []string, cursorLine, cursorCol int) bool {
	textBefore, ok := textBeforeCursor(lines, cursorLine, cursorCol)
	if !ok {
		return false
	}
	trim := strings.TrimSpace(textBefore)
	if strings.HasPrefix(trim, "/") && !strings.Contains(trim, " ") {
		return false
	}
	return true
}

// Compile-time interface checks.
var (
	_ editor.AutocompleteProvider = (*Provider)(nil)
	_ editor.InlineHintProvider   = (*Provider)(nil)
	_ editor.SyncSlashProvider    = (*Provider)(nil)
	_ editor.ForceFileProvider    = (*Provider)(nil)
)

// ---------------------------------------------------------------------------
// Slash
// ---------------------------------------------------------------------------

func (p *Provider) slashSuggestions(textBeforeCursor string, slashStart int) *editor.Suggestions {
	commandText := textBeforeCursor[slashStart:]
	spaceIndex := strings.IndexByte(commandText, ' ')
	cmds := p.snapshotCommands()

	if spaceIndex < 0 {
		prefix := commandText[1:] // strip "/"
		matches := buildSlashCommandCompletions(cmds, strings.ToLower(prefix))
		if len(matches) == 0 {
			return nil
		}
		// Full text-before-cursor preserves leading whitespace for Enter race.
		return &editor.Suggestions{Items: matches, Prefix: textBeforeCursor}
	}

	commandName := commandText[1:spaceIndex]
	argumentText := commandText[spaceIndex+1:]
	cmd := findCommand(cmds, commandName)
	if cmd == nil || cmd.ArgumentCompletions == nil {
		return nil
	}
	items := cmd.ArgumentCompletions(argumentText)
	if len(items) == 0 {
		return nil
	}
	return &editor.Suggestions{Items: items, Prefix: argumentText}
}

// ---------------------------------------------------------------------------
// @ / path
// ---------------------------------------------------------------------------

func (p *Provider) atSuggestions(ctx context.Context, reqID uint64, atPrefix string) *editor.Suggestions {
	kind := parsePathPrefix(atPrefix)
	raw := kind.rawPrefix

	// Outside cwd → immediate-directory prefix listing only (no recursive fuzzy).
	if raw != "" && p.isOutsideCwd(raw) {
		if !p.allowOut {
			return nil
		}
		items := p.getFileSuggestions(ctx, atPrefix)
		if !p.stillCurrent(reqID) || len(items) == 0 {
			return nil
		}
		return &editor.Suggestions{Items: items, Prefix: atPrefix}
	}

	var suggestions []editor.AutocompleteItem
	if raw != "" {
		suggestions = p.getFuzzyFileSuggestions(ctx, raw, kind.isQuotedPrefix)
	} else {
		suggestions = p.getFileSuggestions(ctx, "@")
	}
	if !p.stillCurrent(reqID) {
		return nil
	}
	if len(suggestions) == 0 && raw != "" {
		fallback := p.getFileSuggestions(ctx, atPrefix)
		if !p.stillCurrent(reqID) || len(fallback) == 0 {
			return nil
		}
		return &editor.Suggestions{Items: fallback, Prefix: atPrefix}
	}
	if len(suggestions) == 0 {
		return nil
	}
	return &editor.Suggestions{Items: suggestions, Prefix: atPrefix}
}

func (p *Provider) extractAtPrefix(text string) string {
	if q := extractQuotedPrefix(text); strings.HasPrefix(q, `@"`) {
		return q
	}
	last := findLastDelimiter(text)
	tokenStart := 0
	if last >= 0 {
		tokenStart = last + 1
	}
	if tokenStart < len(text) && text[tokenStart] == '@' {
		return text[tokenStart:]
	}
	return ""
}

func (p *Provider) extractPathPrefixOK(text string, force bool) (string, bool) {
	if q := extractQuotedPrefix(text); q != "" {
		return q, true
	}
	last := findLastDelimiter(text)
	pathPrefix := text
	if last >= 0 {
		pathPrefix = text[last+1:]
	}
	if force {
		return pathPrefix, true
	}
	if strings.Contains(pathPrefix, "/") || strings.HasPrefix(pathPrefix, ".") || strings.HasPrefix(pathPrefix, "~/") {
		return pathPrefix, true
	}
	// Empty token only after a trailing space (not completely empty text).
	if pathPrefix == "" && strings.HasSuffix(text, " ") {
		return pathPrefix, true
	}
	return "", false
}

func (p *Provider) expandHome(filePath string) string {
	return expandHomePath(filePath, p.homeDir)
}

func (p *Provider) isOutsideCwd(rawPrefix string) bool {
	if rawPrefix == "" {
		return false
	}
	var target string
	switch {
	case strings.HasPrefix(rawPrefix, "~"):
		target = p.expandHome(rawPrefix)
	case filepath.IsAbs(rawPrefix) || strings.HasPrefix(rawPrefix, "/"):
		target = rawPrefix
	default:
		target = filepath.Join(p.basePath, rawPrefix)
	}
	target = filepath.Clean(target)
	base := p.basePath
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return true
	}
	if rel == "" || rel == "." {
		return false
	}
	if filepath.IsAbs(rel) {
		return true
	}
	// Windows volume / leading ..
	first := rel
	if i := strings.IndexAny(rel, `/\`); i >= 0 {
		first = rel[:i]
	}
	return first == ".."
}

func (p *Provider) resolveScopedFuzzyQuery(ctx context.Context, rawQuery string) (baseDir, query, displayBase string, ok bool) {
	slash := strings.LastIndex(rawQuery, "/")
	if slash < 0 {
		return "", "", "", false
	}
	displayBase = rawQuery[:slash+1]
	query = rawQuery[slash+1:]
	switch {
	case strings.HasPrefix(displayBase, "~/"):
		baseDir = p.expandHome(displayBase)
	case strings.HasPrefix(displayBase, "/"):
		baseDir = displayBase
	default:
		baseDir = filepath.Join(p.basePath, displayBase)
	}
	info, err := os.Stat(baseDir)
	if err != nil || !info.IsDir() {
		// Allow injected FileSource to answer even if OS path missing.
		if _, listErr := p.files.ListDir(ctx, baseDir); listErr != nil {
			return "", "", "", false
		}
	}
	return baseDir, query, displayBase, true
}

func scopedPathForDisplay(displayBase, relativePath string) string {
	if displayBase == "/" {
		return "/" + relativePath
	}
	return displayBase + relativePath
}

func (p *Provider) getFuzzyFileSuggestions(ctx context.Context, query string, isQuotedPrefix bool) []editor.AutocompleteItem {
	searchPath := p.basePath
	fuzzyQuery := query
	var displayBase string
	scoped := false
	if bd, q, db, ok := p.resolveScopedFuzzyQuery(ctx, query); ok {
		searchPath = bd
		fuzzyQuery = q
		displayBase = db
		scoped = true
	}
	// Refuse recursive walk outside base (OMP outside-cwd short-circuit).
	rel, err := filepath.Rel(p.basePath, filepath.Clean(searchPath))
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		if filepath.Clean(searchPath) != filepath.Clean(p.basePath) {
			return nil
		}
	}

	entries, err := p.files.Discover(ctx, searchPath, fuzzyQuery)
	if err != nil || len(entries) == 0 {
		return nil
	}
	lowerQ := strings.ToLower(fuzzyQuery)
	out := make([]editor.AutocompleteItem, 0, len(entries))
	for _, e := range entries {
		if err := ctx.Err(); err != nil {
			return nil
		}
		entryRel := e.RelPath
		if e.IsDir && !strings.HasSuffix(entryRel, "/") {
			entryRel += "/"
		}
		pathWithoutSlash := strings.TrimSuffix(entryRel, "/")
		normalized := filepath.ToSlash(pathWithoutSlash)
		if hasGitSegment(normalized) {
			continue
		}
		if lowerQ != "" && !fuzzyMatch(lowerQ, strings.ToLower(normalized)) &&
			!fuzzyMatch(lowerQ, strings.ToLower(e.Name)) {
			continue
		}
		displayPath := pathWithoutSlash
		if scoped {
			displayPath = scopedPathForDisplay(displayBase, pathWithoutSlash)
		}
		entryName := e.Name
		if entryName == "" {
			entryName = filepath.Base(pathWithoutSlash)
		}
		completionPath := displayPath
		if e.IsDir {
			completionPath = displayPath + "/"
		}
		value := buildCompletionValue(completionPath, e.IsDir, true, isQuotedPrefix)
		label := entryName
		if e.IsDir {
			label += "/"
		}
		out = append(out, editor.AutocompleteItem{
			Value:       value,
			Label:       label,
			Description: displayPath,
		})
	}
	return out
}

func (p *Provider) getFileSuggestions(ctx context.Context, prefix string) []editor.AutocompleteItem {
	kind := parsePathPrefix(prefix)
	rawPrefix := kind.rawPrefix
	expanded := rawPrefix
	if strings.HasPrefix(expanded, "~") {
		expanded = p.expandHome(expanded)
	}

	isRootPrefix := rawPrefix == "" ||
		rawPrefix == "./" ||
		rawPrefix == "../" ||
		rawPrefix == "~" ||
		rawPrefix == "~/" ||
		rawPrefix == "/" ||
		(kind.isAtPrefix && rawPrefix == "")

	var searchDir, searchPrefix string
	switch {
	case isRootPrefix:
		if strings.HasPrefix(rawPrefix, "~") || strings.HasPrefix(expanded, "/") || filepath.IsAbs(expanded) {
			searchDir = expanded
			if rawPrefix == "~" {
				searchDir = p.homeDir
			}
		} else {
			searchDir = filepath.Join(p.basePath, expanded)
		}
		searchPrefix = ""
	case strings.HasSuffix(rawPrefix, "/"):
		if strings.HasPrefix(rawPrefix, "~") || strings.HasPrefix(expanded, "/") || filepath.IsAbs(expanded) {
			searchDir = expanded
		} else {
			searchDir = filepath.Join(p.basePath, expanded)
		}
		searchPrefix = ""
	default:
		dir := filepath.Dir(expanded)
		file := filepath.Base(expanded)
		if strings.HasPrefix(rawPrefix, "~") || strings.HasPrefix(expanded, "/") || filepath.IsAbs(expanded) {
			searchDir = dir
		} else {
			searchDir = filepath.Join(p.basePath, dir)
		}
		searchPrefix = file
	}

	// Outside-root guard for non-allow.
	if !p.allowOut {
		rel, err := filepath.Rel(p.basePath, filepath.Clean(searchDir))
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			if filepath.Clean(searchDir) != filepath.Clean(p.basePath) {
				return nil
			}
		}
	}

	entries, err := p.files.ListDir(ctx, searchDir)
	if err != nil {
		return nil
	}
	lowerSearch := strings.ToLower(searchPrefix)
	var suggestions []editor.AutocompleteItem
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return nil
		}
		if lowerSearch != "" && !strings.HasPrefix(strings.ToLower(entry.Name), lowerSearch) {
			continue
		}
		if entry.Name == ".git" {
			continue
		}
		isDirectory := entry.IsDir
		name := entry.Name
		displayPrefix := rawPrefix
		var relativePath string

		switch {
		case strings.HasSuffix(displayPrefix, "/"):
			relativePath = displayPrefix + name
		case strings.Contains(displayPrefix, "/"):
			if strings.HasPrefix(displayPrefix, "~/") {
				homeRel := displayPrefix[2:]
				d := filepath.Dir(homeRel)
				if d == "." {
					relativePath = "~/" + name
				} else {
					relativePath = "~/" + filepath.ToSlash(filepath.Join(d, name))
				}
			} else if strings.HasPrefix(displayPrefix, "/") {
				d := filepath.Dir(displayPrefix)
				if d == "/" {
					relativePath = "/" + name
				} else {
					relativePath = filepath.ToSlash(d) + "/" + name
				}
			} else {
				relativePath = filepath.ToSlash(filepath.Join(filepath.Dir(displayPrefix), name))
				if strings.HasPrefix(displayPrefix, "./") && !strings.HasPrefix(relativePath, "./") {
					relativePath = "./" + relativePath
				}
			}
		default:
			if strings.HasPrefix(displayPrefix, "~") {
				relativePath = "~/" + name
			} else {
				relativePath = name
			}
		}

		pathValue := relativePath
		if isDirectory {
			pathValue = relativePath + "/"
		}
		value := buildCompletionValue(pathValue, isDirectory, kind.isAtPrefix, kind.isQuotedPrefix)
		label := name
		if isDirectory {
			label += "/"
		}
		suggestions = append(suggestions, editor.AutocompleteItem{
			Value: value,
			Label: label,
		})
	}

	// Dirs first, then alpha — ListDir already sorts; re-stable for safety.
	sortSuggestionsDirsFirst(suggestions)
	return suggestions
}

func sortSuggestionsDirsFirst(s []editor.AutocompleteItem) {
	// Insertion sort: small N (directory width).
	for i := 1; i < len(s); i++ {
		j := i
		for j > 0 {
			aDir := strings.HasSuffix(s[j].Value, "/") || strings.HasSuffix(s[j].Label, "/")
			bDir := strings.HasSuffix(s[j-1].Value, "/") || strings.HasSuffix(s[j-1].Label, "/")
			less := false
			if aDir != bDir {
				less = aDir
			} else {
				less = s[j].Label < s[j-1].Label
			}
			if !less {
				break
			}
			s[j], s[j-1] = s[j-1], s[j]
			j--
		}
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func textBeforeCursor(lines []string, cursorLine, cursorCol int) (string, bool) {
	if cursorLine < 0 || cursorLine >= len(lines) {
		return "", false
	}
	line := lines[cursorLine]
	if cursorCol < 0 {
		cursorCol = 0
	}
	if cursorCol > len(line) {
		cursorCol = len(line)
	}
	return line[:cursorCol], true
}

func cloneLines(in []string) []string {
	if in == nil {
		return []string{""}
	}
	out := make([]string, len(in))
	copy(out, in)
	return out
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
