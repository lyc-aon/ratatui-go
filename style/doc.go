// Package style provides terminal colors, modifier flags, and incremental
// Style values with Patch semantics.
//
// Color presence on Style is tracked by HasFG/HasBG/HasUnderlineColor so the
// zero Style means "unset / change nothing", while ResetStyle() forces colors
// to Color Reset and clears all modifiers. This matches Ratatui's distinction
// between Style::default() and Style::reset().
//
// This package has no project imports.
package style
