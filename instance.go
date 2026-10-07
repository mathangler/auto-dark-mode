package main

import (
	"time"

	"golang.org/x/sys/windows"
)

// acquireSingleInstance takes a named mutex so only one copy of this tray
// utility runs at a time. Returns false (and does nothing else) if another
// instance is already running. The check is retried briefly: at login another
// instance may be a just-terminating process still holding the mutex.
func acquireSingleInstance() bool {
	name, _ := windows.UTF16PtrFromString("Local\\AutoDarkModeZcode2.Mutex")
	for attempt := 0; attempt < 3; attempt++ {
		m, err := windows.CreateMutex(nil, false, name)
		if err == windows.ERROR_ALREADY_EXISTS {
			windows.CloseHandle(m)
			if attempt < 2 {
				time.Sleep(1500 * time.Millisecond)
				continue
			}
			return false
		}
		if m == 0 {
			// Can't create the mutex; assume single instance is fine.
			return true
		}
		// We own the mutex; keep the handle open for the process lifetime.
		return true
	}
	return false
}
