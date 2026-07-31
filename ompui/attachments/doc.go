// Package attachments resolves interactive @file arguments and inline editor
// @path / @"path with spaces" mentions into a cleaned prompt, protocol-shaped
// image attachments, and structured notices.
//
// Behavior tracks OMP coding-agent helpers:
//   - packages/coding-agent/src/utils/file-mentions.ts (mention extract)
//   - packages/coding-agent/src/cli/file-processor.ts (initial @file args + <file> wrap)
//   - packages/ai/src/types.ts ImageContent ({type,mimeType,data})
//   - packages/utils/src/mime.ts (PNG/JPEG/GIF/WebP magic)
//
// Hardening beyond OMP interactive auto-read (intentional divergences):
//   - regular files only (directories are rejected, not listed)
//   - configurable root / symlink policy (default: stay under root, no symlink escape)
//   - configurable per-file, count, and total decoded-byte limits with notices
//   - binary non-image bytes are noticed, not force-decoded as text
//   - http(s) and other URL forms are never opened
//   - resolved @mentions are removed from the returned prompt; \@ becomes literal @
//   - text contents are inlined with OMP <file name="..."> wrappers; images become
//     base64 ImageContent wire objects
//
// No external processes, no shell, no cwd mutation, no global cache.
package attachments
