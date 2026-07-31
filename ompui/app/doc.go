// Package app is the OMP Go frontend application shell.
//
// It owns the single serialized event loop that multiplexes:
//   - terminal input/resize from ompui/runtime
//   - RPC events/responses from ompui/client
//   - ticks for working indicators
//   - context / signal cancellation
//
// All state mutations and renders run on that loop. Background work
// (RPC Call waiters, open_url) posts results back as app commands and never
// touches render state concurrently.
//
// cmd/omp-tui is the process entrypoint; this package is the library body.
package app
