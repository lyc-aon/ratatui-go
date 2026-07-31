// Package layout provides terminal geometry and constraint-based split layouts.
//
// Core types:
//
//   - Position, Size, Offset, Margin, Rect — integer cell geometry with clamping at zero
//   - Alignment / VerticalAlignment — content alignment enums
//   - Direction, Flex, Constraint — split configuration
//   - Layout — value-style builder with Split / SplitWithSpacers
//
// Constraints are created with Length, Min, Max, Percentage, Ratio, and Fill.
// Layouts are built with Horizontal, Vertical, or New, then configured via
// Margin, Flex, Spacing, and related methods. Builder methods return copies and
// never mutate earlier values; constraint slices passed in are copied.
//
// Split is deterministic and covers zero-size areas, over-constrained totals,
// fill weights, flex distribution, spacing, and margins on both axes.
package layout
