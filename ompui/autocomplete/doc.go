// Package autocomplete implements OMP's CombinedAutocompleteProvider for the
// Go editor: slash-command completion (names, aliases, subcommands, argument
// hints, inline ghost text) and @/path file completion with bounded,
// cancellable filesystem discovery.
//
// The adapter exposes [editor.AutocompleteProvider] plus the optional
// [editor.InlineHintProvider], [editor.SyncSlashProvider], and
// [editor.ForceFileProvider] hooks. File discovery never shells out, never
// mutates the process working directory, and never follows symlink directory
// cycles. Stale async-style results are dropped via per-request IDs and
// context cancellation.
package autocomplete
