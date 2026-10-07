# Auto Dark Mode

A lightweight Windows app that automatically switches the system between dark and
light mode — either on a fixed schedule, or at real sunrise/sunset computed from
your location.

Built with **Go + Wails v2** and a plain **vanilla JS** frontend (no JS framework,
no Node runtime needed at runtime). The only runtime dependency is the WebView2
runtime, which ships with Windows 10/11.

> 📖 完整的设计、功能与使用文档见 [`docs/DESIGN.md`](docs/DESIGN.md)（架构、功能说明、使用指南、配置字段、构建发布、设计决策与已知限制）。A Chinese mirror of this README is available at [`README.zh-CN.md`](README.zh-CN.md); the two correspond section by section.

## Features

- **System-wide switching** — toggles `AppsUseLightTheme` / `SystemUsesLightTheme`
  in `HKCU\...\Themes\Personalize`, so the OS shell and most apps switch live.
- **Two modes** (pick one in Settings):
  - *Sunrise / sunset*: computes daily sunrise and sunset from your coordinates.
  - *Fixed times*: dark from one time to another.
- **Location resolution**, in preference order:
  1. **Auto (GPS → IP)**: Windows Location API first, falling back to IP geolocation.
  2. **Manual coordinates**: type lat/lon yourself.
  3. **City picker**: choose from a built-in table of major cities (no network).
- **System tray icon** — Open Settings / Force Dark / Force Light / Automatic / Quit.
- **Auto-start with Windows** (optional, on/off in Settings).
- **Minimize to tray on close** — closing the window hides to the tray instead of quitting (default on; use tray "Quit" to fully exit).
- **Bilingual (中文 / English)** — follows the system language with a manual override; covers the settings UI, tray menu and status messages.
- **Settings UI follows the OS theme** — the settings window matches the dark/light mode it drives.
- Settings window (Golang backend + vanilla JS UI), config persisted as JSON.

The in-app scheduler checks the clock every 30 seconds, so even during sleep/resume
the theme catches up within half a minute. After a resume the theme is applied
only once the shell has settled again (~8s), so the taskbar and Explorer cannot
miss the switch; if the theme actually changed across the sleep, a one-shot
taskbar-side heal follows ~45s later as a safety net (see Known Issues below).

## Known Issues (fixed)

### After waking from sleep, the taskbar and the Explorer content area don't follow the theme switch

**Status**: ✅ fixed (real root cause found: the resume detection never fired —
see below).

**Historical symptoms (repro steps)**

1. The app runs in **automatic mode** (auto switching enabled, theme driven by the schedule).
2. Put the PC to sleep/hibernate during the **dark period** (the taskbar is dark at sleep time).
3. Wake the PC during the **light period** (the schedule says light should be active now).
4. Actual behavior before the fix:
   - ✅ Third-party apps switch to light;
   - ✅ File Explorer's **menu bar / title bar** switches to light;
   - ❌ The **Windows taskbar** stays dark;
   - ❌ File Explorer's **file-list content area** stays dark.

**Measured behavior (key clue behind the original fix)**

- Clicking tray "**Force Light**" after waking did **not** turn the taskbar light;
- You had to click "**Force Dark**" first and then "**Force Light**" — i.e. make the
  registry theme values really flip dark→light — to restore normal behavior;
- This showed the shell repaints when a broadcast accompanies a real registry
  change, but ignores repeated broadcasts when the values did not change.

**The real root cause (found after several "almost fixed" rounds)**

The whole "wait until the shell is ready, then apply once" design was silently
dead on arrival: it was driven by a resume detection that **never fired**.

- The detection compared the wall clock against `GetTickCount64`, assuming the
  counter freezes during sleep. MSDN documents the opposite: the tick count is
  *biased* — it **includes** time spent in sleep/hibernate (that is exactly the
  difference to `QueryUnbiasedInterruptTime`). Wall gap ≈ tick gap meant "no
  resume", so the first scheduler tick after a wake took the normal path and
  applied the theme **immediately**, while the shell was still resuming — the
  taskbar missed the one real transition broadcast, and since it only repaints
  on real value changes, it stayed stuck for good.
- Two more bugs reinforced this: entering standby (the away-mode *enter*
  notification) also armed the 8-second delayed apply — a biased timer that
  expires during sleep fires **the instant the machine wakes** (documented
  Windows behavior), defeating the delay even when the power-event path
  worked; and `SendMessageTimeout` with `SMTO_ABORTIFHUNG` silently **drops**
  the message for a hung taskbar, so the transition broadcast was never
  redelivered.

**The fix (implemented, see `theme.go` / `app.go` / `tray.go`)**

1. **Resume detection that actually detects resumes**: the scheduler now
   compares the wall clock against `QueryUnbiasedInterruptTime`, whose count
   only advances while the machine is actually running. A wake is genuinely
   detected (even without any `WM_POWERBROADCAST`), and the target theme is
   applied once, **8 seconds after the resume**, when the shell can hear the
   broadcast — the stuck state never forms. **While that delayed apply is
   pending, every scheduler tick defers to it**: Go's overdue ticker
   deadlines all fire back-to-back the instant the machine resumes, and
   without this guard a second tick wrote the theme at wake+0.5s — stealing
   the transition, defeating the delay and disarming the safety net (the
   root cause proven by the diagnostic log in 2026-09).
2. **Sleep-proof delays**: entering standby no longer arms anything (only the
   wake direction of the away-mode notification counts), and the delayed
   apply's wait is itself runtime-based — if the machine falls asleep
   mid-wait, the remainder is re-waited after the real wake.
3. **Robust delivery**: `applyTheme` follows the canonical
   write → `RefreshImmersiveColorPolicyState` → broadcast order, then delivers
   the notifications directly to the taskbar windows (primary **and** the
   secondary taskbars of other monitors) — synchronously, plus as a queued
   `PostMessage` that still reaches a taskbar that was hung at broadcast
   time.
4. **Pixel-gated taskbar-side safety net**: if the delayed wake apply performed a
   real transition (the theme changed across the sleep — the only situation
   in which the taskbar can end up stuck with nothing later able to correct
   it), a heal runs **once, ~45s after the wake**, i.e. at a point where
   the manual two-step remedy was observed to work. **First it samples the
   taskbar's actual pixels** (9×3 grid read via `BitBlt` — `GetPixel` is no
   longer exported): if the bar already shows the target theme confidently,
   the flip is skipped entirely (**zero flash**); only an ambiguous or stuck
   reading runs the flip, which touches only `SystemUsesLightTheme` — the
   side the taskbar follows — so **applications never flash**. After any
   flip, a +2s/+5s delivery ladder re-sends the targeted notifications so
   the restore transition cannot be missed.
5. **Gentler manual heal**: the tray "Force Dark / Force Light / Automatic"
   one-click heal also flips only the system side now — no more whole-desktop
   dark flash when healing a stuck taskbar by hand.

**Trade-off**: after a theme-changing wake the new theme lands about 8 seconds
after resume. When the pixel read is confident the safety net skips the flip
completely — no flash at all; when it is ambiguous (or sampling is
unavailable) the taskbar may briefly switch and switch back once (~3s) and
self-corrects within seconds — never requiring manual action. A single tray
click on "Force Light/Dark" remains the last-resort remedy.

**Historical investigation notes** (kept for reference): the earlier fix rounds
built an increasingly elaborate heal machinery around a resume detection that
never fired, which is why the symptoms persisted (and earlier automatic flips
added a dark↔light flash of the whole desktop). The decisive step was checking
the counter's documented behavior instead of its assumed one.

## Build & Release

Versioning is SemVer with a mandatory bump before every exe build — the rules,
the sync-file inventory and the version history live in
[`docs/VERSIONING.md`](docs/VERSIONING.md); run `build/bump-version.ps1` to bump.

Two distribution forms are supported:

### 1. Portable single EXE

```bash
wails build
```

Produces a single, standalone `build/bin/autodark.exe` — copy it anywhere and
run it. Nothing is installed, no registry/Start-Menu changes are made. It uses
the system WebView2 runtime (shipped with Windows 10/11), so no extra install is
needed.

### 2. Guided installer (NSIS)

```bash
wails build -nsis
```

Produces `build/bin/autodark-amd64-installer.exe`, a Windows installer wizard
that guides the user through installation:
- Pick the installer language on startup (**English / 简体中文**; the dialog appears on every run, reinstalls included, and the choice is remembered for the uninstaller)
- Welcome → installation directory → installing → finish pages
- **If the app is already running, the installer offers to close it and
  continue** (silent installs close it automatically)
- Installs into Program Files, creates **Start Menu** and **desktop** shortcuts
- Checks and installs the WebView2 runtime if missing
- "Run App Dark Mode after install" option on the finish page
- Installs a proper **uninstaller** (closes a running instance first)

#### Installer prerequisites

Building the installer requires NSIS (`makensis`) on `PATH`. Install it with
[`scoop`](https://scoop.sh):

```bash
scoop install nsis
```

### Build everything at once

```bash
build.bat            # portable exe + NSIS installer
build.bat portable   # only the portable exe
build.bat installer  # only the NSIS installer
```

### Live development

For hot reload during development:

```bash
wails dev
```

## Configuration

Persisted at `%APPDATA%\AutoDarkMode\config.json`. On first run the app starts
with automatic switching enabled in **sunrise/sunset** mode using **auto** location.

A lightweight diagnostic log rotates at `%APPDATA%\AutoDarkMode\app.log`
(1 MB × 3 generations), recording wake detection, registry writes, broadcasts,
flips and pixel-gate decisions — the observability that made the 2026-09
root-cause analysis possible. The full event catalog, the healthy-wake
baseline timeline and a symptom → log-signature troubleshooting playbook live
in [`docs/LOGGING.md`](docs/LOGGING.md) — read that first if the sleep/wake
taskbar problem ever reappears.

## Notes

- GPS detection requires the Windows Location permission (the OS may prompt on
  first use). If it's unavailable, the app silently falls back to IP geolocation,
  then manual/city.
- IP geolocation uses `ip-api.com` (no key, non-commercial use).
- Sunrise/sunset uses the NOAA solar algorithm; validated against known values
  for New York (see `solar_test.go`).
