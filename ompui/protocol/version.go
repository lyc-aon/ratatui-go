package protocol

// Protocol major/minor for v1 of the Bun↔Go frontend wire.
const (
	// Major is the current protocol major version.
	// Peers MUST reject connections when majors differ.
	Major = 1

	// Minor is the current protocol minor version.
	// Minors may advance with additive, backward-compatible changes.
	Minor = 0

	// ProtocolName is a stable identifier for diagnostics and hello.
	ProtocolName = "omp-frontend"
)

// Frame size limits.
const (
	// MaxFrameSize is the maximum accepted JSON payload size in bytes
	// (both length-prefixed and JSONL). Larger frames are rejected.
	// 16 MiB covers large get_messages / transcript payloads without
	// allowing unbounded memory growth from a hostile peer.
	MaxFrameSize = 16 << 20

	// MaxJSONLLineSize is an alias of MaxFrameSize for the JSONL codec.
	// Kept as a named constant so call sites can be explicit.
	MaxJSONLLineSize = MaxFrameSize

	// lengthHeaderSize is the fixed size of the length-prefixed header.
	lengthHeaderSize = 4
)
