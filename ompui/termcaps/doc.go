// Package termcaps resolves terminal identity and capability policy.
//
// All decision helpers take explicit env/platform inputs so tests stay
// deterministic. Process-default wrappers (ProcessEnv, Default*) sit only at
// the package edge and read the live process environment / GOOS.
//
// This package is pure: no terminal I/O, no image encoders, no probes.
package termcaps
