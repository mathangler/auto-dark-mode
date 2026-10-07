package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/sys/windows/registry"
)

// diagLog writes one timestamped diagnostic line to app.log under the app's
// config directory. This is the app's observability for sleep/resume theme
// handling: the wake/apply/flip/broadcast paths previously logged nothing,
// which made the stuck-taskbar-after-resume bug impossible to diagnose after
// the fact. Volume is tiny (a few dozen lines per wake), the file rotates at
// 1 MB and only three generations are kept (app.log, app.log.1, app.log.2).
// Never blocks long, never panics — logging must not be able to break the
// theme logic.
var (
	diagMu     sync.Mutex
	diagMaxLen = int64(1 << 20) // rotate app.log at 1 MB
	diagKeep   = 2              // generations kept besides the active file
)

func diagPath() string {
	base, err := os.UserConfigDir()
	if err != nil {
		base = "."
	}
	return filepath.Join(base, "AutoDarkMode", "app.log")
}

func diagLog(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	diagMu.Lock()
	defer diagMu.Unlock()
	p := diagPath()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return
	}
	rotateDiag(p)
	f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	// Millisecond timestamps: the interesting intervals here are seconds.
	fmt.Fprintf(f, "[%s] %s\n", time.Now().Format("2006-01-02 15:04:05.000"), msg)
}

// rotateDiag shifts app.log -> app.log.1 -> app.log.2 when the active file
// has reached diagMaxLen; the oldest generation is dropped. Called with
// diagMu held.
func rotateDiag(p string) {
	st, err := os.Stat(p)
	if err != nil || st.Size() < diagMaxLen {
		return
	}
	os.Remove(fmt.Sprintf("%s.%d", p, diagKeep))
	for i := diagKeep - 1; i >= 1; i-- {
		os.Rename(fmt.Sprintf("%s.%d", p, i), fmt.Sprintf("%s.%d", p, i+1))
	}
	os.Rename(p, p+".1")
}

// themeSnapshot renders the live Personalize registry values so every logged
// decision can be matched against the real registry state afterwards — the
// one fact that separates "the write side went wrong" from "the shell missed
// the repaint".
func themeSnapshot() string {
	k, err := registry.OpenKey(registry.CURRENT_USER, personalizeKey, registry.QUERY_VALUE)
	if err != nil {
		return "apps=? system=? (open failed: " + err.Error() + ")"
	}
	defer k.Close()
	val := func(name string) string {
		v, _, err := k.GetIntegerValue(name)
		if err != nil {
			return "?"
		}
		switch v {
		case 0:
			return "dark"
		case 1:
			return "light"
		default:
			return fmt.Sprintf("%d", v)
		}
	}
	return "apps=" + val("AppsUseLightTheme") + " system=" + val("SystemUsesLightTheme")
}
