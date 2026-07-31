package attachments

// ImageContent is the OMP / rpc-types ImageContent wire shape
// ({ type: "image", mimeType, data }) used on prompt/steer/follow_up.
type ImageContent struct {
	Type     string `json:"type"`
	MimeType string `json:"mimeType"`
	Data     string `json:"data"` // standard base64
}

// Notice codes for structured path failures. Callers may switch on Code;
// Message is human-readable and stable enough for UI display.
const (
	CodeMissing     = "missing"
	CodeNotFile     = "not_file"
	CodeOutsideRoot = "outside_root"
	CodeSymlink     = "symlink"
	CodeTooLarge    = "too_large"
	CodeCountLimit  = "count_limit"
	CodeTotalLimit  = "total_limit"
	CodeBinary      = "binary"
	CodeUnreadable  = "unreadable"
	CodeURL         = "url"
	CodeEmpty       = "empty"
	CodeCancelled   = "cancelled"
	CodeInvalidPath = "invalid_path"
)

// Notice reports one bad/oversize/skipped path without dropping remaining user text.
type Notice struct {
	Path    string `json:"path"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Options configures resolution. Zero values select production defaults.
// Root is the configured project/cwd root; relative paths resolve against it.
type Options struct {
	// Root is the resolution base and default confinement root (required for
	// relative paths). Empty falls back to the process working directory only
	// as a last resort; callers should set it explicitly.
	Root string

	// Home expands leading ~/. Empty uses the current user home when available.
	Home string

	// MaxFiles caps how many distinct paths are fully loaded. 0 → DefaultMaxFiles.
	MaxFiles int

	// MaxFileBytes caps a single non-image file's decoded size. 0 → DefaultMaxFileBytes.
	MaxFileBytes int64

	// MaxImageBytes caps a single image file's decoded size. 0 → DefaultMaxImageBytes.
	MaxImageBytes int64

	// MaxTotalBytes caps the sum of successfully decoded file/image bytes.
	// 0 → DefaultMaxTotalBytes.
	MaxTotalBytes int64

	// AllowSymlinks permits reading through symlinks whose final target still
	// satisfies root policy. Default false: any symlink path is rejected.
	AllowSymlinks bool

	// FollowOutsideRoot allows absolute / ~/ paths whose real location lies
	// outside Root. Default false.
	FollowOutsideRoot bool
}

// Defaults match OMP CLI/auto-read size ceilings and a tight interactive count.
const (
	DefaultMaxFiles      = 32
	DefaultMaxFileBytes  = 5 << 20  // 5 MiB text (OMP MAX_AUTO_READ_TEXT_BYTES)
	DefaultMaxImageBytes = 25 << 20 // 25 MiB image (OMP MAX_AUTO_READ_IMAGE_BYTES)
	DefaultMaxTotalBytes = 50 << 20 // sum ceiling for one resolve call
)

// Result is the production output of a resolve call.
type Result struct {
	// Prompt is the cleaned user text with \@ → @, resolved mention tokens
	// removed, and successful text files appended as OMP <file> blocks.
	Prompt string

	// Images are protocol-shaped base64 image attachments (Type always "image").
	Images []ImageContent

	// Notices lists per-path problems; remaining user text is always preserved.
	Notices []Notice
}

// applyDefaults returns a copy of o with zero numeric fields filled.
func (o Options) applyDefaults() Options {
	if o.MaxFiles <= 0 {
		o.MaxFiles = DefaultMaxFiles
	}
	if o.MaxFileBytes <= 0 {
		o.MaxFileBytes = DefaultMaxFileBytes
	}
	if o.MaxImageBytes <= 0 {
		o.MaxImageBytes = DefaultMaxImageBytes
	}
	if o.MaxTotalBytes <= 0 {
		o.MaxTotalBytes = DefaultMaxTotalBytes
	}
	return o
}
