package app

import (
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// ClipboardHelper handles OS and terminal clipboard interactions.
type ClipboardHelper struct{}

func newClipboardHelper() *ClipboardHelper {
	return &ClipboardHelper{}
}

// ReadText attempts to read UTF-8 text from system clipboard using platform tools.
func (c *ClipboardHelper) ReadText() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		out, err := exec.Command("pbpaste").Output()
		if err == nil {
			return string(out), nil
		}
	case "linux":
		if _, err := exec.LookPath("wl-paste"); err == nil {
			out, err := exec.Command("wl-paste", "--no-newline").Output()
			if err == nil {
				return string(out), nil
			}
		}
		if _, err := exec.LookPath("xclip"); err == nil {
			out, err := exec.Command("xclip", "-selection", "clipboard", "-o").Output()
			if err == nil {
				return string(out), nil
			}
		}
		if _, err := exec.LookPath("xsel"); err == nil {
			out, err := exec.Command("xsel", "--clipboard", "--output").Output()
			if err == nil {
				return string(out), nil
			}
		}
	case "windows":
		out, err := exec.Command("powershell", "-NoProfile", "-Command", "Get-Clipboard").Output()
		if err == nil {
			return strings.ReplaceAll(string(out), "\r\n", "\n"), nil
		}
	}
	return "", fmt.Errorf("no clipboard utility available")
}

// ReadImage attempts to read PNG/JPEG/GIF image data from system clipboard.
// Returns base64 string, mimeType, and error.
func (c *ClipboardHelper) ReadImage() (base64Data string, mimeType string, err error) {
	switch runtime.GOOS {
	case "darwin":
		script := `
			set theFile to (open for access (path to temporary items folder as text & "clipboard_img.png") with write permission)
			try
				set eof theFile to 0
				write (get the clipboard as «class PNGf») to theFile
				close access theFile
				return (path to temporary items folder as text & "clipboard_img.png")
			on error
				close access theFile
				return ""
			end try
		`
		out, e := exec.Command("osascript", "-e", script).Output()
		if e == nil {
			tmpPath := strings.TrimSpace(string(out))
			if tmpPath != "" {
				// Convert HFS path if needed or inspect file
				fileBytes, readErr := os.ReadFile(os.TempDir() + "/clipboard_img.png")
				if readErr == nil && len(fileBytes) > 0 {
					return base64.StdEncoding.EncodeToString(fileBytes), "image/png", nil
				}
			}
		}
	case "linux":
		if _, err := exec.LookPath("wl-paste"); err == nil {
			out, e := exec.Command("wl-paste", "--type", "image/png").Output()
			if e == nil && len(out) > 0 {
				return base64.StdEncoding.EncodeToString(out), "image/png", nil
			}
		}
		if _, err := exec.LookPath("xclip"); err == nil {
			out, e := exec.Command("xclip", "-selection", "clipboard", "-t", "image/png", "-o").Output()
			if e == nil && len(out) > 0 {
				return base64.StdEncoding.EncodeToString(out), "image/png", nil
			}
		}
	}
	return "", "", fmt.Errorf("no clipboard image found")
}

// WriteText copies text to system clipboard and emits OSC 52 sequence if terminal is available.
func (c *ClipboardHelper) WriteText(text string, termWriter func(string)) error {
	var copyErr error

	switch runtime.GOOS {
	case "darwin":
		cmd := exec.Command("pbcopy")
		cmd.Stdin = strings.NewReader(text)
		copyErr = cmd.Run()
	case "linux":
		if _, err := exec.LookPath("wl-copy"); err == nil {
			cmd := exec.Command("wl-copy")
			cmd.Stdin = strings.NewReader(text)
			copyErr = cmd.Run()
		} else if _, err := exec.LookPath("xclip"); err == nil {
			cmd := exec.Command("xclip", "-selection", "clipboard")
			cmd.Stdin = strings.NewReader(text)
			copyErr = cmd.Run()
		} else if _, err := exec.LookPath("xsel"); err == nil {
			cmd := exec.Command("xsel", "--clipboard", "--input")
			cmd.Stdin = strings.NewReader(text)
			copyErr = cmd.Run()
		}
	case "windows":
		cmd := exec.Command("clip")
		cmd.Stdin = strings.NewReader(text)
		copyErr = cmd.Run()
	}

	// Always emit OSC 52 sequence as terminal clipboard backup
	if termWriter != nil {
		encoded := base64.StdEncoding.EncodeToString([]byte(text))
		osc52 := fmt.Sprintf("\x1b]52;c;%s\x07", encoded)
		termWriter(osc52)
	}

	return copyErr
}
