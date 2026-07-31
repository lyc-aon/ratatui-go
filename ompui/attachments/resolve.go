package attachments

import (
	"context"
	"strings"
)

// Resolve processes inline editor text: finds @path / @"path with spaces" /
// @'path' mentions, loads regular files under opts, strips successful mention
// tokens from the prompt, converts \@ to literal @, appends OMP <file> text
// blocks, and returns base64 ImageContent objects for images.
//
// Bad paths produce Notices; remaining user text is never dropped.
// Reads honor ctx cancellation. Never opens URLs, never shells out, never
// mutates the process working directory.
func Resolve(ctx context.Context, text string, opts Options) (Result, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	opts = opts.applyDefaults()
	if opts.Root != "" {
		// normalize root once
		opts.Root = resolveAgainstRoot(opts.Root, "", opts.Home)
	}

	res := Result{}
	if err := ctx.Err(); err != nil {
		return res, err
	}

	mentions := scanMentions(text)
	remove := make([]bool, len(mentions))

	// Dedupe by absolute path while preserving first-seen order for loads.
	type job struct {
		idx      int // mention index
		userPath string
	}
	var jobs []job
	for i, m := range mentions {
		if m.Escaped {
			continue
		}
		jobs = append(jobs, job{idx: i, userPath: m.Raw})
	}

	var (
		usedFiles int
		usedBytes int64
		textParts []string
		// first outcome per abs path: later mentions of same path reuse it
		// (strip on success, skip reload always).
		loadedOK = make(map[string]bool)
	)

	for _, j := range jobs {
		if err := ctx.Err(); err != nil {
			res.Notices = append(res.Notices, Notice{
				Path:    j.userPath,
				Code:    CodeCancelled,
				Message: "cancelled",
			})
			// still clean escapes / prior removals
			res.Prompt = finalizePrompt(text, mentions, remove, textParts)
			return res, err
		}

		// Pre-resolve for dedupe key (best-effort; loadFile re-resolves).
		absKey := resolveAgainstRoot(j.userPath, opts.Root, opts.Home)
		if absKey != "" {
			if ok, exists := loadedOK[absKey]; exists {
				if ok {
					remove[j.idx] = true
				}
				continue
			}
		}

		out := loadFile(ctx, j.userPath, opts, usedFiles, usedBytes)
		if out.notice != nil {
			if out.notice.Path == "" {
				out.notice.Path = j.userPath
			}
			res.Notices = append(res.Notices, *out.notice)
			if absKey != "" {
				loadedOK[absKey] = false
			}
			// Missing / failed mentions stay in the prompt (OMP leaves unresolved
			// prose alone). Only successful loads strip the token.
			continue
		}
		if !out.ok {
			continue
		}
		usedFiles++
		usedBytes += out.bytes
		key := out.absPath
		if key == "" {
			key = absKey
		}
		if key != "" {
			loadedOK[key] = true
		}
		remove[j.idx] = true
		if out.image != nil {
			res.Images = append(res.Images, *out.image)
		}
		if out.textBlock != "" {
			// Images also get an empty <file> ref; text gets full body.
			// For pure images OMP keeps the <file name> marker in the text stream.
			textParts = append(textParts, out.textBlock)
		}
	}

	res.Prompt = finalizePrompt(text, mentions, remove, textParts)
	return res, nil
}

// ResolveInitialArgs processes leading interactive @file CLI-style arguments
// (paths already unquoted, without the leading @). Each path is loaded and
// inlined the same way as a successful mention; there is no surrounding user
// prose to clean. message is optional extra prompt text appended after file blocks.
//
// On missing files this returns notices and continues (unlike OMP CLI process.exit).
func ResolveInitialArgs(ctx context.Context, fileArgs []string, opts Options) (Result, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	opts = opts.applyDefaults()
	if opts.Root != "" {
		opts.Root = resolveAgainstRoot(opts.Root, "", opts.Home)
	}

	res := Result{}
	if err := ctx.Err(); err != nil {
		return res, err
	}

	var (
		usedFiles int
		usedBytes int64
		textParts []string
		seen      = make(map[string]struct{})
	)

	for _, arg := range fileArgs {
		if err := ctx.Err(); err != nil {
			res.Notices = append(res.Notices, Notice{Path: arg, Code: CodeCancelled, Message: "cancelled"})
			res.Prompt = strings.Join(textParts, "")
			return res, err
		}
		arg = strings.TrimSpace(arg)
		if arg == "" {
			continue
		}
		// Allow callers to pass "@foo" or `@\"bar\"` forms.
		if strings.HasPrefix(arg, "@") {
			arg = arg[1:]
			if len(arg) >= 2 {
				if (arg[0] == '"' && arg[len(arg)-1] == '"') || (arg[0] == '\'' && arg[len(arg)-1] == '\'') {
					arg = arg[1 : len(arg)-1]
				}
			}
		}
		absKey := resolveAgainstRoot(arg, opts.Root, opts.Home)
		if absKey != "" {
			if _, ok := seen[absKey]; ok {
				continue
			}
		}

		out := loadFile(ctx, arg, opts, usedFiles, usedBytes)
		if out.notice != nil {
			if out.notice.Path == "" {
				out.notice.Path = arg
			}
			res.Notices = append(res.Notices, *out.notice)
			continue
		}
		if !out.ok {
			continue
		}
		if absKey != "" {
			seen[absKey] = struct{}{}
		}
		if out.absPath != "" {
			seen[out.absPath] = struct{}{}
		}
		usedFiles++
		usedBytes += out.bytes
		if out.image != nil {
			res.Images = append(res.Images, *out.image)
		}
		if out.textBlock != "" {
			textParts = append(textParts, out.textBlock)
		}
	}

	res.Prompt = strings.Join(textParts, "")
	return res, nil
}

// ResolveAll is a convenience for bootstrap: initial @file args plus editor text.
// File-arg blocks are prepended to the cleaned editor prompt (OMP initial-message order).
func ResolveAll(ctx context.Context, fileArgs []string, text string, opts Options) (Result, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	opts = opts.applyDefaults()

	var merged Result
	if len(fileArgs) > 0 {
		a, err := ResolveInitialArgs(ctx, fileArgs, opts)
		if err != nil {
			return a, err
		}
		merged = a
	}
	if text != "" || len(fileArgs) == 0 {
		b, err := Resolve(ctx, text, opts)
		if err != nil {
			// Keep any initial progress.
			merged.Notices = append(merged.Notices, b.Notices...)
			if merged.Prompt == "" {
				merged.Prompt = b.Prompt
			} else if b.Prompt != "" {
				merged.Prompt = merged.Prompt + b.Prompt
			}
			merged.Images = append(merged.Images, b.Images...)
			return merged, err
		}
		if merged.Prompt == "" {
			merged.Prompt = b.Prompt
		} else if b.Prompt != "" {
			merged.Prompt = merged.Prompt + b.Prompt
		} else {
			// file args only already set prompt
		}
		// When both produced text, OMP concatenates fileText then message.
		// If Resolve also appended <file> blocks inside Prompt, order is:
		//   initial file blocks + cleaned user text (+ its file blocks).
		merged.Images = append(merged.Images, b.Images...)
		merged.Notices = append(merged.Notices, b.Notices...)
	}
	return merged, nil
}

func finalizePrompt(text string, mentions []mention, remove []bool, fileBlocks []string) string {
	cleaned := rewritePrompt(text, mentions, remove)
	if len(fileBlocks) == 0 {
		return cleaned
	}
	blocks := strings.Join(fileBlocks, "")
	if cleaned == "" {
		return blocks
	}
	// Keep user prose first, then inlined file bodies (mentions were mid-sentence;
	// OMP actually injects a separate fileMention message — we inline for the
	// Go frontend prompt wire which only has message+images).
	if strings.HasSuffix(cleaned, "\n") {
		return cleaned + blocks
	}
	return cleaned + "\n" + blocks
}
