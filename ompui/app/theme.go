package app

import (
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"sync"

	"github.com/lyc-aon/ratatui-go/ompui/component"
	"github.com/lyc-aon/ratatui-go/ompui/latex"
	"github.com/lyc-aon/ratatui-go/ompui/media"
	"github.com/lyc-aon/ratatui-go/ompui/protocol"
	"github.com/lyc-aon/ratatui-go/ompui/renderer"
	ompruntime "github.com/lyc-aon/ratatui-go/ompui/runtime"
	"github.com/lyc-aon/ratatui-go/ompui/termcaps"
	"github.com/lyc-aon/ratatui-go/ompui/view"
)

// themeBundle holds the active view theme. highlight/mermaid are owned by
// view.NewTheme — do not wire them here.
type themeBundle struct {
	theme      view.Theme
	appearance view.Appearance
	colorMode  view.ColorMode
	// remoteOwned is set after a valid Bun theme_sync. While true, terminal
	// appearance ticks must not rebuild/clobber the synced theme.
	remoteOwned bool
}

func buildTheme(snap *termcaps.Snapshot, appearance view.Appearance) themeBundle {
	return buildThemeWithPalette(snap, appearance, nil, false)
}

func buildThemeWithPalette(snap *termcaps.Snapshot, appearance view.Appearance, pal *view.Palette, remoteOwned bool) themeBundle {
	if v := strings.TrimSpace(os.Getenv("NO_COLOR")); v != "" {
		latex.SetColorMode(latex.ColorNone)
		return themeBundle{
			theme:       view.MonoTheme(),
			appearance:  appearance,
			colorMode:   view.ColorNone,
			remoteOwned: remoteOwned,
		}
	}
	mode := view.ColorTrue
	if snap != nil && !snap.TrueColor {
		mode = view.Color256
	} else {
		colorterm := strings.ToLower(os.Getenv("COLORTERM"))
		if !strings.Contains(colorterm, "truecolor") && !strings.Contains(colorterm, "24bit") {
			term := os.Getenv("TERM")
			if strings.Contains(term, "256color") {
				mode = view.Color256
			} else if term == "dumb" {
				mode = view.ColorNone
			}
		}
	}
	switch mode {
	case view.ColorNone:
		latex.SetColorMode(latex.ColorNone)
	case view.Color256:
		latex.SetColorMode(latex.ColorANSI256)
	default:
		latex.SetColorMode(latex.ColorTrueColor)
	}
	if appearance != view.AppearanceLight {
		appearance = view.AppearanceDark
	}
	to := view.ThemeOptions{
		Appearance: appearance,
		ColorMode:  mode,
		Palette:    pal,
	}
	if snap != nil {
		to.Hyperlinks = snap.Hyperlinks
		to.TextSizing = snap.TextSizing
	}
	return themeBundle{
		theme:       view.NewTheme(to),
		appearance:  appearance,
		colorMode:   mode,
		remoteOwned: remoteOwned,
	}
}

// strictThemeHex reports whether s is exactly #RRGGBB (hex digits only).
func strictThemeHex(s string) bool {
	if len(s) != 7 || s[0] != '#' {
		return false
	}
	for i := 1; i < 7; i++ {
		c := s[i]
		if (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F') {
			continue
		}
		return false
	}
	return true
}

// mergeThemeSyncPalette starts from appearance defaults and overlays valid hex fields.
func mergeThemeSyncPalette(appearance view.Appearance, p *protocol.ThemeSyncPalette) view.Palette {
	base := view.DarkPalette()
	if appearance == view.AppearanceLight {
		base = view.LightPalette()
	}
	if p == nil {
		return base
	}
	set := func(dst *string, src string) {
		if strictThemeHex(src) {
			*dst = src
		}
	}
	set(&base.Text, p.Text)
	set(&base.Muted, p.Muted)
	set(&base.Dim, p.Dim)
	set(&base.Accent, p.Accent)
	set(&base.Success, p.Success)
	set(&base.Error, p.Error)
	set(&base.Warning, p.Warning)
	set(&base.Thinking, p.Thinking)
	set(&base.Code, p.Code)
	set(&base.Border, p.Border)
	set(&base.User, p.User)
	return base
}

// resolveThemeSyncAppearance picks light/dark from payload fields, else keeps current.
// ok is false when neither appearance nor a light/dark name was provided.
func resolveThemeSyncAppearance(p protocol.ThemeSyncPayload, current view.Appearance) (view.Appearance, bool) {
	switch strings.ToLower(strings.TrimSpace(p.Appearance)) {
	case "light":
		return view.AppearanceLight, true
	case "dark":
		return view.AppearanceDark, true
	}
	switch strings.ToLower(strings.TrimSpace(p.Name)) {
	case "light":
		return view.AppearanceLight, true
	case "dark":
		return view.AppearanceDark, true
	}
	return current, false
}

// applyThemeSync rebuilds the view theme from a Bun theme_sync frame.
// NO_COLOR stays authoritative. Unknown name-only frames leave the theme alone.
// A successful apply marks the bundle remoteOwned so terminal appearance ticks
// cannot clobber the synced palette.
func (a *App) applyThemeSync(p protocol.ThemeSyncPayload) {
	appearance, appearanceSet := resolveThemeSyncAppearance(p, a.themes.appearance)
	if p.Palette == nil && !appearanceSet {
		a.logf("theme_sync name=%q ignored (no appearance/palette)", p.Name)
		return
	}

	var snap *termcaps.Snapshot
	if a.term != nil {
		snap = a.term.Capabilities()
	}

	// NO_COLOR: keep mono theme; still record appearance when provided.
	if v := strings.TrimSpace(os.Getenv("NO_COLOR")); v != "" {
		a.themes = buildThemeWithPalette(snap, appearance, nil, true)
		a.applyThemeAll()
		a.requestRender(renderer.ReasonForce)
		return
	}

	merged := mergeThemeSyncPalette(appearance, p.Palette)
	a.themes = buildThemeWithPalette(snap, appearance, &merged, true)
	a.applyThemeAll()
	a.requestRender(renderer.ReasonForce)
}

func appearanceFromRuntime(a ompruntime.Appearance) view.Appearance {
	switch a {
	case ompruntime.AppearanceLight:
		return view.AppearanceLight
	default:
		return view.AppearanceDark
	}
}

// imageCache owns media.Image instances keyed by view.ImageRequest.Key so
// graphics ids and budget slots survive re-renders. The adapter closure is
// stable; Options must not be rebuilt each frame.
type imageCache struct {
	mu       sync.Mutex
	byKey    map[string]*media.Image
	budget   *media.ImageBudget
	protocol func() termcaps.ImageProtocol
	cells    func() termcaps.CellDimensions
}

func newImageCache(budget *media.ImageBudget, protocol func() termcaps.ImageProtocol, cells func() termcaps.CellDimensions) *imageCache {
	return &imageCache{
		byKey:    make(map[string]*media.Image),
		budget:   budget,
		protocol: protocol,
		cells:    cells,
	}
}

func (c *imageCache) Adapter() view.ImageAdapter {
	return func(req view.ImageRequest) component.Component {
		if req.Base64 == "" || c == nil {
			return nil
		}
		proto := c.protocol()
		if proto == "" {
			return nil
		}
		c.mu.Lock()
		defer c.mu.Unlock()
		if img, ok := c.byKey[req.Key]; ok {
			return img
		}
		p := proto
		opts := media.Options{
			MaxWidthCells:  120,
			MaxHeightCells: 24,
			Budget:         c.budget,
			ImageKey:       req.Key,
			Protocol:       &p,
			Cell:           c.cells(),
			Filename:       req.Filename,
		}
		img := media.NewImage(req.Base64, req.MIMEType, media.Theme{
			FallbackColor: func(s string) string { return "\x1b[2m" + s + "\x1b[22m" },
		}, opts, nil)
		c.byKey[req.Key] = img
		return img
	}
}

func (c *imageCache) Clear() {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.byKey = make(map[string]*media.Image)
	c.mu.Unlock()
}

func hostPaths(coreDir string) (path, home string) {
	path = coreDir
	if path == "" {
		if w, err := os.Getwd(); err == nil {
			path = w
		}
	}
	if u, err := user.Current(); err == nil {
		home = u.HomeDir
	} else if h, err := os.UserHomeDir(); err == nil {
		home = h
	}
	if path != "" && !filepath.IsAbs(path) {
		if abs, err := filepath.Abs(path); err == nil {
			path = abs
		}
	}
	return path, home
}

func defaultFooterInfo(coreDir string) view.FooterInfo {
	path, home := hostPaths(coreDir)
	return view.FooterInfo{Path: path, Home: home}
}

func defaultWelcomeInfo(coreDir string) view.WelcomeInfo {
	path, home := hostPaths(coreDir)
	return view.WelcomeInfo{
		AppName: "omp",
		Version: Version,
		Path:    path,
		Home:    home,
		Tip:     "enter sends · esc aborts · ctrl+o expands tools · ctrl+c clears",
	}
}

func bold(s string) string {
	if s == "" {
		return s
	}
	return "\x1b[1m" + s + "\x1b[22m"
}

func danger(s string) string {
	if s == "" {
		return s
	}
	return "\x1b[31m" + s + "\x1b[39m"
}
