package main

import (
	"golang.org/x/sys/windows"
)

// effectiveLang resolves the language to display: an explicit config value
// ("en"/"zh") wins; otherwise it follows the system UI language.
func effectiveLang(cfg Config) string {
	if cfg.Lang == "en" || cfg.Lang == "zh" {
		return cfg.Lang
	}
	return detectSystemLang()
}

// detectSystemLang returns "zh" when the system UI language is Chinese,
// otherwise "en".
func detectSystemLang() string {
	kernel32 := windows.NewLazySystemDLL("kernel32.dll")
	proc := kernel32.NewProc("GetUserDefaultUILanguage")
	// The low 10 bits of the LANGID are the primary language (0x04 = Chinese).
	langID, _, _ := proc.Call()
	primary := langID & 0x03ff
	if primary == 0x04 {
		return "zh"
	}
	return "en"
}

// dict holds the handful of user-facing strings surfaced by the backend
// (tray labels and error messages). The settings UI carries its own dict.
var dict = map[string]map[string]string{
	"en": {
		"tray.enabled":      "Enabled",
		"tray.openSettings": "Open Settings",
		"tray.forceDark":    "Force Dark",
		"tray.forceLight":   "Force Light",
		"tray.automatic":    "Automatic",
		"tray.quit":         "Quit",

		"err.invalidTheme":        "invalid theme %q",
		"err.locationFailed":      "no location available",
		"err.applyFailed":         "apply failed",
		"err.autostartFailed":     "autostart failed",
		"err.resolveFailed":       "location resolution failed",
		"err.noLocation":          "Cannot determine your current location. Please set it manually.",
		"err.unknownCity":         "unknown city",
		"err.gpsFailed":           "GPS lookup failed",
	},
	"zh": {
		"tray.enabled":      "已启用",
		"tray.openSettings": "打开设置",
		"tray.forceDark":    "强制深色",
		"tray.forceLight":   "强制浅色",
		"tray.automatic":    "自动",
		"tray.quit":         "退出",

		"err.invalidTheme":        "无效的主题 %q",
		"err.locationFailed":      "无法获取位置",
		"err.applyFailed":         "应用主题失败",
		"err.autostartFailed":     "设置开机自启失败",
		"err.resolveFailed":       "位置解析失败",
		"err.noLocation":          "无法获取当前位置，请手动设置。",
		"err.unknownCity":         "未知城市",
		"err.gpsFailed":           "GPS 定位失败",
	},
}

// T returns the localized string for lang ("en"/"zh") and key, falling back
// to English when the key is missing.
func T(lang, key string) string {
	if m, ok := dict[lang]; ok {
		if s, ok := m[key]; ok {
			return s
		}
	}
	return dict["en"][key]
}
