// Package interact implements retained line-string interaction components for
// the OMP Go frontend: Input, ScrollView, SelectList, SettingsList, TabBar,
// Box, Spinner/Loader, and OverlayStack.
//
// Components implement ompui/component contracts and consume canonical
// event.Event values. They perform no terminal I/O and never write to a TTY.
// Generation is bumped only when rendered content changes so Container and the
// renderer can memoize.
//
// Key navigation mirrors OMP defaults (up/down/page/home/end, tab/shift-tab,
// enter/escape, optional j/k, mouse wheel/click where exposed). OverlayStack
// owns z-order, modal geometry, and focus transfer/restoration; overlay rows
// are never part of the transcript frame and must not be committed to
// scrollback by the renderer.
//
// Imports: component, event, ansitext, fuzzy, killring, textutil.
package interact
