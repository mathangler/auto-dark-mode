package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// The updater compares the running build against the newest release that the
// CI published on GitHub (a release only exists after a tag push built and
// uploaded the assets, so "there is a release" is the same as "there is a
// build"). It can then download the NSIS installer with live progress and run
// it silently, relaunching the app afterwards.

const (
	updateReleaseAPI  = "https://api.github.com/repos/mathangler/auto-dark-mode/releases/latest"
	updateReleasePage = "https://github.com/mathangler/auto-dark-mode/releases/latest"
	updateEventName   = "update:changed"
	updateCheckEvery  = 6 * time.Hour
)

// UpdateInfo is both the state payload and what the UI renders.
type UpdateInfo struct {
	State      string `json:"state"` // idle|checking|uptodate|available|downloading|ready|installing|error
	Current    string `json:"current"`
	Latest     string `json:"latest"`
	Notes      string `json:"notes"`
	ReleaseURL string `json:"releaseUrl"`
	AssetName  string `json:"assetName"`
	AssetURL   string `json:"assetUrl"`
	AssetID    int64  `json:"-"`
	Size       int64  `json:"size"`
	Progress   int    `json:"progress"`
	Error      string `json:"error"`
	Published  string `json:"published"`

	filePath string
}

type ghRelease struct {
	TagName     string `json:"tag_name"`
	Body        string `json:"body"`
	HTMLURL     string `json:"html_url"`
	PublishedAt string `json:"published_at"`
	Assets      []struct {
		ID                 int64  `json:"id"`
		Name               string `json:"name"`
		Size               int64  `json:"size"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

// updateState is guarded by updateMu; downloads and checks never run twice.
var (
	updateMu      sync.Mutex
	updateCurrent UpdateInfo
	updateBusy    bool
)

func (a *App) emitUpdate() {
	updateMu.Lock()
	info := updateCurrent
	updateMu.Unlock()
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, updateEventName, info)
	}
}

// GetUpdateInfo returns the last known update state (used by GetState).
func (a *App) GetUpdateInfo() UpdateInfo {
	updateMu.Lock()
	defer updateMu.Unlock()
	return updateCurrent
}

// CheckForUpdate asks GitHub for the latest published release and compares its
// version with this build.
func (a *App) CheckForUpdate() (UpdateInfo, error) {
	updateMu.Lock()
	if updateBusy {
		info := updateCurrent
		updateMu.Unlock()
		return info, nil
	}
	updateBusy = true
	updateCurrent = UpdateInfo{State: "checking", Current: Version, Progress: 0}
	updateMu.Unlock()
	a.emitUpdate()

	info, err := fetchLatestRelease(Version)

	updateMu.Lock()
	updateBusy = false
	if err != nil {
		updateCurrent = UpdateInfo{State: "error", Current: Version, Error: err.Error(), ReleaseURL: updateReleasePage}
	} else {
		updateCurrent = info
	}
	out := updateCurrent
	updateMu.Unlock()

	if err != nil {
		diagLog("update: check failed: %v", err)
	} else {
		diagLog("update: check current=%s latest=%s state=%s", out.Current, out.Latest, out.State)
	}
	a.emitUpdate()
	return out, err
}

// updateAPIURL allows a local mirror during development/tests.
func updateAPIURL() string {
	if v := os.Getenv("AUTODARK_UPDATE_API"); v != "" {
		return v
	}
	return updateReleaseAPI
}

func fetchLatestRelease(current string) (UpdateInfo, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest(http.MethodGet, updateAPIURL(), nil)
	if err != nil {
		return UpdateInfo{}, err
	}
	req.Header.Set("User-Agent", "AutoDarkMode/"+current)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := client.Do(req)
	if err != nil {
		return UpdateInfo{}, fmt.Errorf("无法连接 GitHub：%v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return UpdateInfo{}, fmt.Errorf("GitHub 返回 %d", resp.StatusCode)
	}

	var rel ghRelease
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&rel); err != nil {
		return UpdateInfo{}, fmt.Errorf("无法解析发布信息：%v", err)
	}

	latest := strings.TrimPrefix(strings.TrimSpace(rel.TagName), "v")
	info := UpdateInfo{
		Current:    current,
		Latest:     latest,
		Notes:      truncate(rel.Body, 600),
		ReleaseURL: rel.HTMLURL,
		Published:  rel.PublishedAt,
	}
	if info.ReleaseURL == "" {
		info.ReleaseURL = updateReleasePage
	}

	// Prefer the NSIS installer, fall back to any executable asset.
	pick := -1
	for i, as := range rel.Assets {
		name := strings.ToLower(as.Name)
		if strings.HasSuffix(name, ".exe") && strings.Contains(name, "installer") {
			pick = i
			break
		}
	}
	if pick < 0 {
		for i, as := range rel.Assets {
			if strings.HasSuffix(strings.ToLower(as.Name), ".exe") {
				pick = i
				break
			}
		}
	}
	if pick >= 0 {
		info.AssetName = rel.Assets[pick].Name
		info.AssetURL = rel.Assets[pick].BrowserDownloadURL
		info.AssetID = rel.Assets[pick].ID
		info.Size = rel.Assets[pick].Size
	}

	if latest == "" {
		info.State = "error"
		info.Error = "发布信息缺少版本号"
		return info, nil
	}
	if compareVersions(latest, current) > 0 && info.AssetURL != "" {
		info.State = "available"
	} else {
		info.State = "uptodate"
	}
	return info, nil
}

// DownloadUpdate downloads the installer asset, emitting progress events.
func (a *App) DownloadUpdate() error {
	updateMu.Lock()
	info := updateCurrent
	if updateBusy || info.State != "available" || info.AssetURL == "" {
		updateMu.Unlock()
		return nil
	}
	updateBusy = true
	updateCurrent.State = "downloading"
	updateCurrent.Progress = 0
	updateMu.Unlock()
	a.emitUpdate()

	go func() {
		err := downloadAsset(info, func(pct int) {
			updateMu.Lock()
			updateCurrent.Progress = pct
			updateMu.Unlock()
			a.emitUpdate()
		})

		updateMu.Lock()
		updateBusy = false
		if err != nil {
			updateCurrent.State = "error"
			updateCurrent.Error = err.Error()
			updateCurrent.Progress = 0
		} else {
			updateCurrent.State = "ready"
			updateCurrent.Progress = 100
		}
		out := updateCurrent
		updateMu.Unlock()

		if err != nil {
			diagLog("update: download failed: %v", err)
		} else {
			diagLog("update: downloaded %s -> %s", out.AssetName, out.filePath)
		}
		a.emitUpdate()
	}()
	return nil
}

func downloadAsset(info UpdateInfo, onProgress func(int)) error {
	// The release page URL (github.com) is the canonical download, but on some
	// networks that host is unreachable while api.github.com works. The API
	// asset endpoint serves the same bytes, so try it as a fallback.
	urls := []string{info.AssetURL}
	if info.AssetID > 0 {
		urls = append(urls, fmt.Sprintf("https://api.github.com/repos/mathangler/auto-dark-mode/releases/assets/%d", info.AssetID))
	}
	var lastErr error
	for _, u := range urls {
		if u == "" {
			continue
		}
		if err := downloadFrom(u, info, onProgress); err != nil {
			lastErr = err
			diagLog("update: download attempt failed via %s: %v", u, err)
			onProgress(0)
			continue
		}
		return nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("没有可用的下载地址")
	}
	return lastErr
}

func downloadFrom(url string, info UpdateInfo, onProgress func(int)) error {
	dir := filepath.Join(os.TempDir(), "AutoDarkModeUpdate")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	dest := filepath.Join(dir, filepath.Base(info.AssetName))

	client := &http.Client{Timeout: 0} // large file: rely on the request context
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "AutoDarkMode/"+Version)
	// Ask the API endpoint for the raw bytes (ignored by the CDN URL).
	req.Header.Set("Accept", "application/octet-stream")
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("下载失败：%v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("下载失败：HTTP %d", resp.StatusCode)
	}

	tmp := dest + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	total := resp.ContentLength
	if total <= 0 {
		total = info.Size
	}
	buf := make([]byte, 64*1024)
	var done int64
	last := time.Now()
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := f.Write(buf[:n]); werr != nil {
				f.Close()
				os.Remove(tmp)
				return werr
			}
			done += int64(n)
			if total > 0 && time.Since(last) > 120*time.Millisecond {
				last = time.Now()
				onProgress(int(done * 100 / total))
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			f.Close()
			os.Remove(tmp)
			return fmt.Errorf("下载中断：%v", rerr)
		}
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if done < 1024 {
		os.Remove(tmp)
		return fmt.Errorf("下载内容异常（%d 字节）", done)
	}
	if err := os.Rename(tmp, dest); err != nil {
		os.Remove(tmp)
		return err
	}

	updateMu.Lock()
	updateCurrent.filePath = dest
	updateMu.Unlock()
	onProgress(100)
	return nil
}

// InstallUpdate runs the downloaded installer silently. The app quits first;
// a small helper script waits for the installer to finish and relaunches the
// installed application (the installer only auto-runs the app in interactive
// mode, not with /S).
func (a *App) InstallUpdate() error {
	updateMu.Lock()
	info := updateCurrent
	updateMu.Unlock()
	if info.filePath == "" {
		return fmt.Errorf("没有已下载的更新包")
	}
	if _, err := os.Stat(info.filePath); err != nil {
		return fmt.Errorf("更新包不存在：%v", err)
	}

	updateMu.Lock()
	updateCurrent.State = "installing"
	updateMu.Unlock()
	a.emitUpdate()
	diagLog("update: installing %s", info.filePath)

	helper := filepath.Join(os.TempDir(), "autodark-update.cmd")
	logPath := filepath.Join(os.TempDir(), "autodark-update.log")
	script := "@echo off\r\n" +
		"echo [%date% %time%] helper start >> \"" + logPath + "\"\r\n" +
		"ping -n 3 127.0.0.1 >nul\r\n" +
		"\"" + info.filePath + "\" /S\r\n" +
		"echo [%date% %time%] installer exit %ERRORLEVEL% >> \"" + logPath + "\"\r\n" +
		"if exist \"%APPDATA%\\Microsoft\\Windows\\Start Menu\\Programs\\Auto Dark Mode.lnk\" (\r\n" +
		"  start \"\" \"%APPDATA%\\Microsoft\\Windows\\Start Menu\\Programs\\Auto Dark Mode.lnk\"\r\n" +
		") else (\r\n" +
		"  if exist \"%LOCALAPPDATA%\\Programs\\Auto Dark Mode\\autodark.exe\" start \"\" \"%LOCALAPPDATA%\\Programs\\Auto Dark Mode\\autodark.exe\"\r\n" +
		")\r\n" +
		"del \"%~f0\"\r\n"
	if err := os.WriteFile(helper, []byte(script), 0o600); err != nil {
		return err
	}

	// `start` detaches the helper from this process tree: spawning cmd directly
	// made the child die together with the app, so the installer never ran.
	cmd := exec.Command("cmd.exe", "/C", "start", "", "/MIN", helper)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x00000008} // DETACHED_PROCESS
	if err := cmd.Start(); err != nil {
		return err
	}

	go func() {
		time.Sleep(500 * time.Millisecond)
		runtime.Quit(a.ctx)
	}()
	return nil
}

// autoCheckUpdates runs a quiet check shortly after startup and then every few
// hours. Failures stay silent: an offline machine must not nag the user.
func (a *App) autoCheckUpdates() {
	time.Sleep(10 * time.Second)
	if a.GetConfig().AutoUpdateCheck {
		if _, err := a.CheckForUpdate(); err == nil {
			updateMu.Lock()
			state := updateCurrent.State
			updateMu.Unlock()
			if state == "available" {
				diagLog("update: new version available, notifying UI")
				// Test hook: lets the download/progress path be exercised
				// without UI automation. No effect unless explicitly set.
				if os.Getenv("AUTODARK_UPDATE_AUTODL") == "1" {
					time.Sleep(2 * time.Second)
					a.DownloadUpdate()
					// Test hook: install as soon as the download completes.
					if os.Getenv("AUTODARK_UPDATE_AUTOINSTALL") == "1" {
						go func() {
							for i := 0; i < 300; i++ {
								time.Sleep(time.Second)
								if a.GetUpdateInfo().State == "ready" {
									a.InstallUpdate()
									return
								}
							}
						}()
					}
				}
			}
		}
	}
	ticker := time.NewTicker(updateCheckEvery)
	defer ticker.Stop()
	for range ticker.C {
		if a.GetConfig().AutoUpdateCheck {
			a.CheckForUpdate()
		}
	}
}

// compareVersions compares dotted numeric versions ("2.0.0" vs "2.0.1").
func compareVersions(a, b string) int {
	pa := splitVersion(a)
	pb := splitVersion(b)
	for i := 0; i < 3; i++ {
		if pa[i] < pb[i] {
			return -1
		}
		if pa[i] > pb[i] {
			return 1
		}
	}
	return 0
}

func splitVersion(v string) [3]int {
	var out [3]int
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		v = v[:i]
	}
	for i, part := range strings.Split(v, ".") {
		if i >= 3 {
			break
		}
		n, _ := strconv.Atoi(strings.TrimSpace(part))
		out[i] = n
	}
	return out
}

func truncate(s string, n int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) <= n {
		return string(r)
	}
	return string(r[:n]) + "…"
}
