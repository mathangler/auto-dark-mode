import './style.css';

import {
    GetConfig,
    GetVersion,
    SaveConfig,
    GetCities,
    GetState,
    DetectLocation,
    SetEnabled,
    SetManualTheme,
    SetLang,
    SetCloseTray,
    SetAutoStart,
    OpenRepository,
    OpenReleases,
    OpenLogsFolder,
    CheckForUpdate,
    DownloadUpdate,
    InstallUpdate,
} from '../wailsjs/go/main/App';
import {
    EventsOn,
    WindowMinimise,
    WindowToggleMaximise,
    WindowIsMaximised,
    WindowHide,
    Quit,
} from '../wailsjs/runtime/runtime';

/* ==========================================================================
   Auto Dark Mode — Liquid Glass UI (v2.0.0)
   The window is frameless and translucent; the title bar is drawn here and
   dragging is delegated to Wails (WM_NCLBUTTONDOWN/HTCAPTION), which keeps
   native Aero Snap working.
   ========================================================================== */

// --- i18n ----------------------------------------------------------------

const I18N = {
    en: {
        autoSwitch: 'Auto switching',
        autoSave: 'Auto-saved',
        schedule: 'Schedule',
        modeSolarShort: 'Sunrise / sunset',
        modeFixedShort: 'Fixed times',
        sunriseToLight: 'Sunrise → light',
        sunsetToDark: 'Sunset → dark',
        latitude: 'Latitude',
        longitude: 'Longitude',
        todayDaylight: 'Daylight today',
        darkDuration: 'Dark hours',
        lightDuration: 'Light hours',
        currentPeriod: 'Current period',
        labelDarkFrom: 'Dark from',
        labelLightFrom: 'Light from',
        location: 'Location',
        locAutoShort: 'Auto',
        locManualShort: 'Manual',
        locCityShort: 'City',
        locSource: 'Source',
        selectedCity: 'Selected',
        searchCity: 'Search city…',
        noCity: 'No matching city',
        detectNow: 'Detect now',
        detecting: 'Locating…',
        locatingPermission: 'Requesting location permission…',
        locatingGetting: 'Getting your location…',
        locatingIpDenied: 'Location permission denied — falling back to IP…',
        locatingIpFallback: 'Unable to get your location — falling back to IP…',
        general: 'General',
        switchNow: 'Switch now',
        themeAuto: 'Automatic',
        themeDark: 'Dark',
        themeLight: 'Light',
        language: 'Language',
        followSystem: 'Follow system',
        launchAtLogin: 'Launch at login',
        closeToTray: 'Minimize to tray on close',
        about: 'About',
        version: 'Version',
        repoLinks: 'Repository & updates',
        openRepo: 'GitHub repository',
        checkUpdates: 'Check for updates',
        openLogs: 'Open log folder',
        openReleases: 'Release page',
        updateTitle: 'Updates',
        updateAvailable: 'New version',
        updateLatest: 'Up to date',
        downloading: 'Downloading',
        updateReady: 'Ready to install',
        downloadInstall: 'Download & install',
        installRestart: 'Install & restart',
        installing: 'Installing…',
        updateFailed: 'Check failed',
        autoUpdateCheck: 'Check for updates automatically',
        diagnostics: 'Diagnostic log',
        saved: 'Saved',
        saveFailed: "Couldn't save settings",
        toDark: 'Dark in',
        toLight: 'Light in',
        currentLight: 'Light mode',
        currentDark: 'Dark mode',
        statusCoords: 'Coordinates',
        sunrise: 'Sunrise',
        sunset: 'Sunset',
        srcAuto: 'Auto (GPS → IP)',
        srcManual: 'Manual',
        srcCity: 'City',
        srcGps: 'GPS',
        srcIp: 'IP',
        winMin: 'Minimize',
        winMax: 'Maximize',
        winRestore: 'Restore',
        winClose: 'Close',
        daylightH: 'h',
        daylightM: 'm',
        hours: 'h',
        minutes: 'min',
        nothingYet: '—',
    },
    zh: {
        autoSwitch: '自动切换',
        autoSave: '自动保存',
        schedule: '计划',
        modeSolarShort: '日出 / 日落',
        modeFixedShort: '固定时间',
        sunriseToLight: '日出 → 浅色',
        sunsetToDark: '日落 → 深色',
        latitude: '纬度',
        longitude: '经度',
        todayDaylight: '今日日照',
        darkDuration: '深色时长',
        lightDuration: '浅色时长',
        currentPeriod: '当前时段',
        labelDarkFrom: '深色从',
        labelLightFrom: '浅色从',
        location: '位置',
        locAutoShort: '自动',
        locManualShort: '手动',
        locCityShort: '城市',
        locSource: '来源',
        selectedCity: '已选',
        searchCity: '搜索城市…',
        noCity: '没有匹配的城市',
        detectNow: '立即探测',
        detecting: '定位中…',
        locatingPermission: '正在申请定位权限…',
        locatingGetting: '正在获取位置…',
        locatingIpDenied: '未授予定位权限，尝试通过 IP 确定位置…',
        locatingIpFallback: '无法获取当前位置，尝试通过 IP 确定位置…',
        general: '通用',
        switchNow: '立即切换',
        themeAuto: '自动',
        themeDark: '深色',
        themeLight: '浅色',
        language: '语言',
        followSystem: '跟随系统',
        launchAtLogin: '开机自启动',
        closeToTray: '关闭时最小化到托盘',
        about: '关于',
        version: '版本',
        repoLinks: '仓库与更新',
        openRepo: 'GitHub 仓库',
        checkUpdates: '检查更新',
        openLogs: '打开日志目录',
        openReleases: '发布页',
        updateTitle: '更新',
        updateAvailable: '发现新版本',
        updateLatest: '已是最新版本',
        downloading: '正在下载',
        updateReady: '已下载，可安装',
        downloadInstall: '下载并安装',
        installRestart: '安装并重启',
        installing: '正在安装…',
        updateFailed: '检查失败',
        autoUpdateCheck: '自动检查更新',
        diagnostics: '诊断日志',
        saved: '已保存',
        saveFailed: '保存失败',
        toDark: '距离深色还有',
        toLight: '距离浅色还有',
        currentLight: '浅色模式',
        currentDark: '深色模式',
        statusCoords: '坐标',
        sunrise: '日出',
        sunset: '日落',
        srcAuto: '自动定位（GPS → IP）',
        srcManual: '手动',
        srcCity: '城市',
        srcGps: 'GPS',
        srcIp: 'IP',
        winMin: '最小化',
        winMax: '最大化',
        winRestore: '还原',
        winClose: '关闭',
        daylightH: '小时',
        daylightM: '分',
        hours: '小时',
        minutes: '分钟',
        nothingYet: '—',
    },
};

let lang = 'en';
let cfg = null;
let cities = [];
let status = null;
let updateInfo = null;
let detecting = false;

const t = (key) => (I18N[lang] && I18N[lang][key]) || I18N.en[key] || key;

// --- helpers -------------------------------------------------------------

const $ = (sel, root = document) => root.querySelector(sel);
const $$ = (sel, root = document) => Array.from(root.querySelectorAll(sel));
const cap = (s) => s.charAt(0).toUpperCase() + s.slice(1);

function fmtCoord(v, kind) {
    if (v === undefined || v === null || isNaN(v)) return '?';
    const d = Math.abs(v);
    const dir = kind === 'lat' ? (v >= 0 ? 'N' : 'S') : (v >= 0 ? 'E' : 'W');
    return d.toFixed(3) + '°' + dir;
}

function parseHM(s) {
    if (!s) return null;
    const m = /^(\d{1,2}):(\d{2})$/.exec(s);
    if (!m) return null;
    return { h: Number(m[1]), m: Number(m[2]) };
}

function minutesOfDay(s) {
    const hm = parseHM(s);
    return hm ? hm.h * 60 + hm.m : null;
}

function fmtDuration(ms) {
    const total = Math.max(0, Math.round(ms / 60000));
    const h = Math.floor(total / 60);
    const m = total % 60;
    if (lang === 'zh') {
        if (h <= 0) return `${m} 分钟`;
        if (m === 0) return `${h} 小时`;
        return `${h} 小时 ${m} 分`;
    }
    if (h <= 0) return `${m} min`;
    if (m === 0) return `${h} ${t('hours')}`;
    return `${h}${t('hours')} ${m}${t('minutes')}`;
}

function fmtClockHM(s) {
    return s || t('nothingYet');
}

// --- theme / backdrop ----------------------------------------------------

function applyTheme(theme) {
    document.documentElement.dataset.theme = theme === 'light' ? 'light' : 'dark';
}

function applyBackdrop(kind) {
    document.documentElement.dataset.backdrop = kind === 'acrylic' ? 'acrylic' : 'none';
}

const lightPref = window.matchMedia('(prefers-color-scheme: light)');
if (lightPref.addEventListener) {
    lightPref.addEventListener('change', () => applyTheme(lightPref.matches ? 'light' : 'dark'));
}
applyTheme(lightPref.matches ? 'light' : 'dark');

// --- language ------------------------------------------------------------

function applyLanguage(effective) {
    if (effective && I18N[effective]) lang = effective;
    document.documentElement.lang = lang === 'zh' ? 'zh-CN' : 'en';

    $$('[data-i18n]').forEach((node) => { node.textContent = t(node.dataset.i18n); });
    $$('[data-i18n-placeholder]').forEach((node) => node.setAttribute('placeholder', t(node.dataset.i18nPlaceholder)));
    $$('[data-i18n-title]').forEach((node) => node.setAttribute('title', t(node.dataset.i18nTitle)));
    $$('[data-i18n-aria]').forEach((node) => node.setAttribute('aria-label', t(node.dataset.i18nAria)));

    const det = $('#detectBtnText');
    if (det && !detecting) det.textContent = t('detectNow');

    // The language picker's own label must show the *selection*, not the
    // "follow system" placeholder the generic i18n pass just wrote.
    if (selectState.lang) {
        const idx = selectState.lang.items.findIndex((i) => i.classList.contains('is-on'));
        if (idx >= 0) setSelect('lang', idx);
    }

    // City labels differ per language: re-render, keeping the filter.
    const input = $('#cityInput');
    if (input) renderCityList(input.value);
    renderCityPicked();

    renderHero();
    if (status) renderStatus();
    // The updater line is composed from translated fragments (not a plain
    // data-i18n node), so re-render it from the cached state on a language
    // change; otherwise it stayed in the previous language until the next
    // update event.
    if (updateInfo) renderUpdate(updateInfo);
    // Button labels may have changed width (zh vs en): re-measure the pills.
    requestAnimationFrame(layoutAllSegs);
}

// --- status rendering ----------------------------------------------------

function renderStatus() {
    if (!status) return;
    applyTheme(status.currentTheme);
    if (status.backdrop) applyBackdrop(status.backdrop);
    if (status.lang && status.lang !== lang) applyLanguage(status.lang);

    if (status.manualTheme && status.manualTheme !== (cfg && cfg.manualTheme)) {
        if (cfg) cfg.manualTheme = status.manualTheme;
        setSeg('theme', ['auto', 'dark', 'light'].indexOf(status.manualTheme));
    }
    if (status.enabled !== undefined && $('#enabled')) {
        $('#enabled').checked = !!status.enabled;
    }
    if (status.autoStart !== undefined && $('#autoStart')) $('#autoStart').checked = !!status.autoStart;
    if (status.closeToTray !== undefined && $('#closeTray')) $('#closeTray').checked = !!status.closeToTray;

    // Badge
    const dark = status.currentTheme === 'dark';
    const badge = $('#themeBadge');
    badge.classList.toggle('is-dark', dark);
    $('#themeBadgeText').textContent = dark ? t('currentDark') : t('currentLight');

    // Schedule read-outs
    $('#schRise').textContent = fmtClockHM(status.sunrise);
    $('#schSet').textContent = fmtClockHM(status.sunset);
    const rise = minutesOfDay(status.sunrise);
    const set = minutesOfDay(status.sunset);
    const dayLen = (rise !== null && set !== null) ? (set - rise + 1440) % 1440 : 0;
    $('#daylight').textContent = dayLen > 0
        ? `${Math.floor(dayLen / 60)} ${t('daylightH')} ${String(dayLen % 60).padStart(2, '0')} ${t('daylightM')}`
        : t('nothingYet');

    // Location read-outs
    const loc = status.location || {};
    $('#locSourceVal').textContent = loc.source ? t('src' + cap(loc.source)) : t('nothingYet');
    $('#locCoordsVal').textContent = loc.error
        ? loc.error
        : (loc.lat !== undefined ? `${fmtCoord(loc.lat, 'lat')}, ${fmtCoord(loc.lon, 'lon')}` : t('nothingYet'));
    if ($('#locHint')) {
        $('#locHint').textContent = loc.source ? t('src' + cap(loc.source)) : '';
    }

    // Error note
    const err = status.lastError || '';
    const errEl = $('#noteError');
    if (err) {
        $('#noteErrorText').textContent = err;
        errEl.classList.add('is-on');
    } else if (!detecting) {
        errEl.classList.remove('is-on');
    }

    renderHero();
}

function renderHero() {
    if (!status) return;

    const now = new Date();
    const mode = status.mode || (cfg && cfg.mode) || 'solar';
    document.documentElement.dataset.mode = mode;

    // Countdown to the next transition ("YYYY-MM-DD HH:MM", local time).
    // The label describes the NEXT target: if we are dark now, the upcoming
    // switch is to light (and vice versa) — using desiredTheme here said
    // "dark in 7 hours" at 00:25, which was backwards.
    const nextIsLight = status.currentTheme === 'dark';
    $('#cdLabel').textContent = nextIsLight ? t('toLight') : t('toDark');

    let cd = t('nothingYet');
    if (status.nextTransition) {
        const m = /^(\d{4})-(\d{2})-(\d{2}) (\d{2}):(\d{2})$/.exec(status.nextTransition);
        if (m) {
            const target = new Date(+m[1], +m[2] - 1, +m[3], +m[4], +m[5], 0, 0);
            cd = fmtDuration(target - now);
        }
    }
    $('#cdValue').textContent = cd;

    // Chips
    const chips = [];
    if (mode === 'solar') {
        chips.push(`<span class="chip">${t('sunrise')} <b>${fmtClockHM(status.sunrise)}</b></span>`);
        chips.push(`<span class="chip">${t('sunset')} <b>${fmtClockHM(status.sunset)}</b></span>`);
    } else {
        chips.push(`<span class="chip">${t('labelDarkFrom')} <b>${status.darkStart || '19:00'}</b></span>`);
        chips.push(`<span class="chip">${t('labelLightFrom')} <b>${status.lightStart || '07:00'}</b></span>`);
    }
    const loc = status.location || {};
    if (loc.source) chips.push(`<span class="chip">${t('locSource')} <b>${t('src' + cap(loc.source))}</b></span>`);
    if (loc.lat !== undefined && !loc.error) {
        chips.push(`<span class="chip">${t('statusCoords')} <b>${fmtCoord(loc.lat, 'lat')}, ${fmtCoord(loc.lon, 'lon')}</b></span>`);
    }
    $('#heroChips').innerHTML = chips.join('');

    // Sun arc (solar) / timeline (fixed). The solar day may wrap local
    // midnight (a far-away location seen from this clock), so both the day
    // and the night interval are computed modulo 24h.
    const rise = minutesOfDay(status.sunrise);
    const set = minutesOfDay(status.sunset);
    if (mode === 'solar' && rise !== null && set !== null && rise !== set) {
        const nowMin = now.getHours() * 60 + now.getMinutes() + now.getSeconds() / 60;
        const dayLen = (set - rise + 1440) % 1440;
        const sinceRise = (nowMin - rise + 1440) % 1440;
        const night = sinceRise >= dayLen;

        let p;
        if (!night) {
            p = dayLen > 0 ? sinceRise / dayLen : 0;
        } else {
            const nightLen = 1440 - dayLen;
            const sinceSet = (nowMin - set + 1440) % 1440;
            p = nightLen > 0 ? sinceSet / nightLen : 0;
        }
        p = Math.max(0, Math.min(1, p));

        const cx = 180, rx = 162, ry = 96, baseY = 112;
        const ang = Math.PI * p;
        const x = cx - rx * Math.cos(ang);
        const y = baseY - ry * Math.sin(ang);

        document.documentElement.dataset.night = night ? '1' : '0';

        const sun = $('#arcSun');
        const halo = $('#arcHalo');
        const moon = $('#arcMoonG');
        if (night) {
            if (sun) sun.style.display = 'none';
            if (halo) halo.style.display = 'none';
            if (moon) {
                moon.style.display = '';
                moon.setAttribute('transform', `translate(${x.toFixed(2)} ${y.toFixed(2)})`);
            }
        } else {
            if (sun) {
                sun.style.display = '';
                sun.setAttribute('cx', x.toFixed(2));
                sun.setAttribute('cy', y.toFixed(2));
                sun.setAttribute('fill', '#FFD166');
                sun.setAttribute('r', '5.5');
            }
            if (halo) {
                halo.style.display = '';
                halo.setAttribute('cx', x.toFixed(2));
                halo.setAttribute('cy', y.toFixed(2));
            }
            if (moon) moon.style.display = 'none';
        }

        const prog = $('#arcProg');
        if (prog) {
            const len = prog.getTotalLength ? prog.getTotalLength() : 500;
            // Stop the lit segment at the icon's edge instead of its centre:
            // a line running into the disc (and through the crescent's notch)
            // looked like it impaled the sun/moon.
            const gap = night ? 14 : 11;
            const drawn = Math.max(0, len * p - gap);
            prog.style.strokeDasharray = len;
            prog.style.strokeDashoffset = len - drawn;
            prog.style.strokeLinecap = 'round';
            prog.style.stroke = night ? 'url(#arcGradNight)' : 'url(#arcGrad)';
        }

        // Labels follow the same direction as the arc.
        const leftLbl = $('#arcLeftLabel');
        const rightLbl = $('#arcRightLabel');
        if (night) {
            leftLbl.textContent = t('sunset');
            leftLbl.dataset.i18n = 'sunset';
            rightLbl.textContent = t('sunrise');
            rightLbl.dataset.i18n = 'sunrise';
            $('#arcLeftTime').textContent = fmtClockHM(status.sunset);
            $('#arcRightTime').textContent = fmtClockHM(status.sunrise);
        } else {
            leftLbl.textContent = t('sunrise');
            leftLbl.dataset.i18n = 'sunrise';
            rightLbl.textContent = t('sunset');
            rightLbl.dataset.i18n = 'sunset';
            $('#arcLeftTime').textContent = fmtClockHM(status.sunrise);
            $('#arcRightTime').textContent = fmtClockHM(status.sunset);
        }
    } else {
        $('#arcLeftTime').textContent = fmtClockHM(status.sunrise);
        $('#arcRightTime').textContent = fmtClockHM(status.sunset);
    }

    // Fixed-time timeline
    if (mode === 'fixed') {
        const darkStart = minutesOfDay(status.darkStart || '19:00') ?? 19 * 60;
        const lightStart = minutesOfDay(status.lightStart || '07:00') ?? 7 * 60;
        const bar = $('#tlBar');
        if (bar) {
            const stops = [];
            if (lightStart < darkStart) {
                stops.push(`#2b3040 0%`, `#2b3040 ${(lightStart / 14.4).toFixed(2)}%`);
                stops.push(`#ffd166 ${(lightStart / 14.4).toFixed(2)}%`, `#ffd166 ${(darkStart / 14.4).toFixed(2)}%`);
                stops.push(`#2b3040 ${(darkStart / 14.4).toFixed(2)}%`, `#2b3040 100%`);
            } else {
                stops.push(`#ffd166 0%`, `#ffd166 ${(darkStart / 14.4).toFixed(2)}%`);
                stops.push(`#2b3040 ${(darkStart / 14.4).toFixed(2)}%`, `#2b3040 ${(lightStart / 14.4).toFixed(2)}%`);
                stops.push(`#ffd166 ${(lightStart / 14.4).toFixed(2)}%`, `#ffd166 100%`);
            }
            bar.style.background = `linear-gradient(90deg, ${stops.join(', ')})`;
        }
        const nowMin = now.getHours() * 60 + now.getMinutes();
        const nowEl = $('#tlNow');
        if (nowEl) nowEl.style.left = ((nowMin / 1440) * 100).toFixed(2) + '%';
        $('#tlTick0').textContent = status.lightStart || '07:00';
        $('#tlTick1').textContent = status.darkStart || '19:00';

        // Keep the card's height stable across panes: the fixed pane shows the
        // same number of rows as the solar one, with the two durations.
        const darkLen = ((lightStart - darkStart) + 1440) % 1440;
        const lightLen = 1440 - darkLen;
        const fmtLen = (mins) => `${Math.floor(mins / 60)} ${t('hours')} ${mins % 60} ${t('minutes')}`;
        if ($('#fixedDarkDur')) $('#fixedDarkDur').textContent = fmtLen(darkLen);
        if ($('#fixedLightDur')) $('#fixedLightDur').textContent = fmtLen(lightLen);
    }

    // Current period line
    const cur = $('#curPeriod');
    if (cur) {
        const period = status.currentTheme === 'dark' ? t('currentDark') : t('currentLight');
        cur.textContent = period;
    }
}

// --- form ----------------------------------------------------------------

function renderForm() {
    if (!cfg) return;
    $('#enabled').checked = !!cfg.enabled;
    $('#autoStart').checked = !!cfg.autoStart;
    $('#closeTray').checked = cfg.closeToTray === true;
    if ($('#autoUpdateCheck')) $('#autoUpdateCheck').checked = cfg.autoUpdateCheck !== false;

    const mode = cfg.mode === 'fixed' ? 'fixed' : 'solar';
    setSeg('mode', mode === 'solar' ? 0 : 1);
    showPane('mode', mode);

    const src = cfg.locationSource || 'auto';
    const srcIdx = ['auto', 'manual', 'city'].indexOf(src);
    setSeg('loc', srcIdx < 0 ? 0 : srcIdx);
    showPane('loc', srcIdx < 0 ? 'auto' : src);

    $('#darkStart').value = cfg.darkStart || '19:00';
    $('#lightStart').value = cfg.lightStart || '07:00';
    $('#lat').value = (cfg.lat && cfg.lat !== 0) ? cfg.lat : '';
    $('#lon').value = (cfg.lon && cfg.lon !== 0) ? cfg.lon : '';

    const langVal = cfg.lang || '';
    const langIdx = langVal === 'zh' ? 2 : langVal === 'en' ? 1 : 0;
    setSelect('lang', langIdx);

    const manual = cfg.manualTheme || 'auto';
    setSeg('theme', ['auto', 'dark', 'light'].indexOf(manual));

    renderCityPicked();
}

function readConfig() {
    const c = Object.assign({}, cfg);
    c.enabled = $('#enabled').checked;
    c.autoStart = $('#autoStart').checked;
    c.closeToTray = $('#closeTray').checked;
    c.autoUpdateCheck = $('#autoUpdateCheck') ? $('#autoUpdateCheck').checked : true;
    c.mode = segValue('mode') || 'solar';
    c.darkStart = $('#darkStart').value || '19:00';
    c.lightStart = $('#lightStart').value || '07:00';
    c.locationSource = segValue('loc') || 'auto';
    if (c.locationSource === 'manual') {
        c.lat = parseFloat($('#lat').value) || 0;
        c.lon = parseFloat($('#lon').value) || 0;
        c.city = '';
    } else if (c.locationSource === 'city') {
        c.lat = 0;
        c.lon = 0;
        c.city = $('#cityPicked').dataset.city || '';
    } else {
        c.lat = 0;
        c.lon = 0;
        c.city = '';
    }
    c.lang = cfg.lang || '';
    c.manualTheme = segValue('theme') || 'auto';
    return c;
}

// --- auto-save -----------------------------------------------------------

let saveTimer = 0;

function scheduleSave() {
    clearTimeout(saveTimer);
    saveTimer = setTimeout(async () => {
        try {
            cfg = readConfig();
            await SaveConfig(cfg);
            toast(t('saved'));
        } catch (e) {
            console.error('SaveConfig failed', e);
            toast(t('saveFailed'), true);
        }
        await refresh();
    }, 300);
}

// --- segments / select / combo ------------------------------------------

const segState = {};

function setupSeg(name, onChange) {
    const seg = document.querySelector(`[data-seg="${name}"]`);
    if (!seg) return;
    const btns = $$(`.seg__btn`, seg);
    seg.style.setProperty('--n', btns.length);
    segState[name] = { seg, btns };
    btns.forEach((b, i) => {
        b.addEventListener('click', () => {
            setSeg(name, i);
            if (onChange) onChange(b.dataset.val, i);
        });
    });
    seg.addEventListener('keydown', (e) => {
        const cur = btns.findIndex((b) => b.classList.contains('is-on'));
        if (e.key === 'ArrowRight' || e.key === 'ArrowDown') {
            e.preventDefault();
            const i = (cur + 1) % btns.length;
            setSeg(name, i);
            if (onChange) onChange(btns[i].dataset.val, i);
            btns[i].focus();
        }
        if (e.key === 'ArrowLeft' || e.key === 'ArrowUp') {
            e.preventDefault();
            const i = (cur - 1 + btns.length) % btns.length;
            setSeg(name, i);
            if (onChange) onChange(btns[i].dataset.val, i);
            btns[i].focus();
        }
    });
}

function setSeg(name, index) {
    const st = segState[name];
    if (!st || index < 0) return;
    st.btns.forEach((b, i) => {
        const on = i === index;
        b.classList.toggle('is-on', on);
        b.setAttribute('aria-checked', on ? 'true' : 'false');
    });
    layoutSeg(name);
}

// The thumb is positioned from the ACTIVE BUTTON'S measured geometry rather
// than a percentage: percentage widths inside the grid container resolved
// against the grid area, which made the pill a quarter of the expected width
// (and hid the active label). Measuring is immune to that and to font/i18n
// width changes.
function layoutSeg(name) {
    const st = segState[name];
    if (!st) return;
    const thumb = $('.seg__thumb', st.seg);
    const idx = st.btns.findIndex((b) => b.classList.contains('is-on'));
    if (!thumb || idx < 0) return;
    const segRect = st.seg.getBoundingClientRect();
    const btnRect = st.btns[idx].getBoundingClientRect();
    if (btnRect.width === 0) return;
    thumb.style.width = btnRect.width + 'px';
    thumb.style.transform = `translateX(${btnRect.left - segRect.left - 3}px)`;
}

function layoutAllSegs() {
    Object.keys(segState).forEach(layoutSeg);
}

function segValue(name) {
    const st = segState[name];
    if (!st) return '';
    const on = st.btns.find((b) => b.classList.contains('is-on'));
    return on ? on.dataset.val : '';
}

const selectState = {};

function setupSelect(name, onChange) {
    const root = document.querySelector(`[data-select="${name}"]`);
    if (!root) return;
    const btn = $('.select__btn', root);
    const items = $$('.select__item', root);
    const menu = $('.select__menu', root);
    selectState[name] = { root, btn, items, menu };
    btn.addEventListener('click', () => {
        const open = !root.classList.contains('is-open');
        // Open upwards when the menu would fall outside the window.
        if (open) {
            const r = btn.getBoundingClientRect();
            const h = menu.scrollHeight || 120;
            root.classList.toggle('is-up', r.bottom + h + 14 > window.innerHeight);
        }
        root.classList.toggle('is-open', open);
        btn.setAttribute('aria-expanded', open ? 'true' : 'false');
    });
    items.forEach((it, i) => {
        it.addEventListener('click', () => {
            setSelect(name, i);
            closeSelect(name);
            if (onChange) onChange(it.dataset.val, i);
        });
    });
}

function setSelect(name, index) {
    const st = selectState[name];
    if (!st || index < 0) return;
    st.items.forEach((it, i) => {
        const on = i === index;
        it.classList.toggle('is-on', on);
        it.setAttribute('aria-selected', on ? 'true' : 'false');
    });
    const label = $('.select__label', st.root);
    if (label && st.items[index]) label.textContent = st.items[index].textContent;
}

function closeSelect(name) {
    const st = selectState[name];
    if (!st) return;
    st.root.classList.remove('is-open');
    st.btn.setAttribute('aria-expanded', 'false');
}

// --- city combobox -------------------------------------------------------

let activeCity = -1;
let filteredCities = [];

function cityLabel(c) {
    return lang === 'zh' ? (c.nameZh || c.name) : c.name;
}

function citySub(c) {
    const other = lang === 'zh' ? c.name : (c.nameZh || '');
    const coords = `${c.lat.toFixed(2)}, ${c.lon.toFixed(2)}`;
    return other ? `${other} · ${coords}` : coords;
}

function renderCityList(query = '') {
    const list = $('#cityList');
    if (!list) return;
    const q = String(query || '').trim().toLowerCase();
    filteredCities = (cities || []).filter((c) =>
        !q ||
        c.name.toLowerCase().includes(q) ||
        (c.nameZh || '').includes(q) ||
        `${c.lat}`.includes(q) ||
        `${c.lon}`.includes(q)
    );
    list.innerHTML = filteredCities.length
        ? filteredCities.map((c, i) => `<div class="combo__opt" role="option" data-i="${i}">
             <span>${cityLabel(c)}<span class="sub">${lang === 'zh' ? c.name : (c.nameZh || '')}</span></span>
             <span class="sub">${c.lat.toFixed(2)}, ${c.lon.toFixed(2)}</span>
           </div>`).join('')
        : `<div class="combo__empty">${t('noCity')}</div>`;
    activeCity = filteredCities.length ? 0 : -1;
    highlightCity();
}

function highlightCity() {
    const list = $('#cityList');
    if (!list) return;
    $$('.combo__opt', list).forEach((n, i) => n.classList.toggle('is-active', i === activeCity));
    const on = $('.combo__opt.is-active', list);
    if (on) on.scrollIntoView({ block: 'nearest' });
}

function openCombo(open) {
    const combo = $('#cityCombo');
    const input = $('#cityInput');
    if (!combo) return;
    if (open) {
        const r = input.getBoundingClientRect();
        const h = $('#cityList').scrollHeight || 160;
        combo.classList.toggle('is-up', r.bottom + h + 14 > window.innerHeight);
    }
    combo.classList.toggle('is-open', open);
    if (input) input.setAttribute('aria-expanded', open ? 'true' : 'false');
}

function pickCity(i) {
    if (i < 0 || i >= filteredCities.length) return;
    const c = filteredCities[i];
    const picked = $('#cityPicked');
    picked.dataset.city = c.name;
    picked.textContent = `${cityLabel(c)} · ${c.name}`;
    $('#cityInput').value = cityLabel(c);
    openCombo(false);
    if (cfg) {
        cfg.city = c.name;
        cfg.locationSource = 'city';
    }
    // Commit immediately (not through the debounce): the hero must re-render
    // with the new sunrise/sunset as soon as the pick is made.
    clearTimeout(saveTimer);
    saveTimer = setTimeout(async () => {
        try {
            cfg = readConfig();
            await SaveConfig(cfg);
            toast(t('saved'));
        } catch (e) {
            console.error('SaveConfig failed', e);
            toast(t('saveFailed'), true);
        }
        await refresh();
    }, 0);
}

function renderCityPicked() {
    const picked = $('#cityPicked');
    if (!picked || !cfg) return;
    const c = (cities || []).find((x) => x.name === cfg.city);
    if (c) {
        picked.dataset.city = c.name;
        picked.textContent = `${cityLabel(c)} · ${c.name}`;
        if ($('#cityInput')) $('#cityInput').value = cityLabel(c);
    } else {
        picked.dataset.city = '';
        picked.textContent = t('nothingYet');
    }
}

// --- panes ---------------------------------------------------------------

function showPane(group, val) {
    const prefix = group === 'mode' ? 'mode' : 'loc';
    $$(`[data-pane^="${prefix}-"]`).forEach((p) => {
        p.classList.toggle('is-on', p.dataset.pane === `${prefix}-${val}`);
    });
    if (group === 'mode') document.documentElement.dataset.mode = val;
}

// --- toast / notes -------------------------------------------------------

let toastTimer = 0;

function toast(text, isError = false) {
    const el = $('#toast');
    if (!el) return;
    $('#toastText').textContent = text;
    el.classList.toggle('error', !!isError);
    el.classList.add('is-on');
    clearTimeout(toastTimer);
    toastTimer = setTimeout(() => el.classList.remove('is-on'), 1800);
}

function setLocatingNote(text) {
    const note = $('#noteInfo');
    if (!note) return;
    $('#noteInfoText').textContent = text;
    note.classList.add('is-on');
}

function clearLocatingNote() {
    const note = $('#noteInfo');
    if (note) note.classList.remove('is-on');
}

// --- state ---------------------------------------------------------------

async function refresh() {
    try {
        status = await GetState();
        renderStatus();
        if (status && status.update) renderUpdate(status.update);
    } catch (e) {
        console.error('GetState failed', e);
    }
}

// renderUpdate paints the updater row: status text, action button label and
// the download progress bar (fed by the update:changed events).
function renderUpdate(info) {
    if (!info) return;
    updateInfo = info;
    const line = $('#updateStatus');
    const btn = $('#btnUpdate');
    const bar = $('#updateProgress');
    const fill = $('#updateProgressFill');
    if (!line || !btn) return;

    const ver = info.latest ? ' v' + info.latest : '';
    let label = t('checkUpdates');
    let disabled = false;
    let showBar = false;
    let pct = 0;

    switch (info.state) {
        case 'checking':
            line.textContent = t('checkUpdates') + '…';
            disabled = true;
            break;
        case 'available':
            line.textContent = `${t('updateAvailable')}${ver}`;
            label = t('downloadInstall');
            break;
        case 'downloading':
            pct = Math.max(0, Math.min(100, info.progress || 0));
            line.textContent = `${t('downloading')} ${pct}%`;
            label = t('downloading') + '…';
            disabled = true;
            showBar = true;
            break;
        case 'ready':
            line.textContent = `${t('updateReady')}${ver}`;
            label = t('installRestart');
            break;
        case 'installing':
            line.textContent = t('installing');
            disabled = true;
            break;
        case 'uptodate':
            line.textContent = `${t('updateLatest')} v${info.current || ''}`.trim();
            break;
        case 'error':
            line.textContent = info.error || t('updateFailed');
            break;
        default:
            line.textContent = '—';
    }

    btn.textContent = label;
    btn.disabled = disabled;
    if (bar && fill) {
        bar.hidden = !showBar;
        fill.style.width = pct + '%';
    }
}

// --- window controls -----------------------------------------------------

async function updateMaxIcon() {
    try {
        const max = await WindowIsMaximised();
        const btn = $('#btnMax');
        if (!btn) return;
        btn.dataset.i18nAria = max ? 'winRestore' : 'winMax';
        btn.setAttribute('aria-label', max ? t('winRestore') : t('winMax'));
        btn.innerHTML = max
            ? `<svg viewBox="0 0 10 10"><rect x="1.6" y="3.4" width="5" height="5" rx=".6"/><path d="M3.4 3.4V1.6h5v5H6.6"/></svg>`
            : `<svg viewBox="0 0 10 10"><rect x="2.2" y="2.2" width="5.6" height="5.6" rx=".6"/></svg>`;
    } catch (e) { /* ignore */ }
}

// --- template ------------------------------------------------------------

function template() {
    return `
    <div class="scene" aria-hidden="true"><i class="orb o1"></i><i class="orb o2"></i><i class="orb o3"></i></div>

    <div class="window">
      <header class="titlebar">
        <div class="tb-drag" id="tbDrag">
          <svg class="tb-icon" viewBox="0 0 32 32" aria-hidden="true">
            <defs>
              <linearGradient id="tbG" x1="0" y1="0" x2="1" y2="1">
                <stop offset="0" stop-color="#cfe4ff"/><stop offset=".45" stop-color="#7aa7ff"/>
                <stop offset="1" stop-color="#2f3f86"/>
              </linearGradient>
              <radialGradient id="tbS"><stop offset="0" stop-color="#FFE29A"/><stop offset="1" stop-color="#FFB340"/></radialGradient>
            </defs>
            <rect width="32" height="32" rx="9" fill="url(#tbG)"/>
            <circle cx="21" cy="11" r="4.2" fill="url(#tbS)"/>
            <path d="M13.6 9.2a7.2 7.2 0 1 0 8.6 9.1 5.8 5.8 0 0 1-8.6-9.1z" fill="#F3F6FF" opacity=".95"/>
            <rect x=".5" y=".5" width="31" height="31" rx="8.5" fill="none" stroke="#fff" stroke-opacity=".45"/>
          </svg>
          <span class="tb-title">Auto Dark Mode</span>
        </div>
        <div class="tb-btns">
          <button class="tb-btn" id="btnMin" data-i18n-aria="winMin"><svg viewBox="0 0 10 10"><path d="M2 5h6"/></svg></button>
          <button class="tb-btn" id="btnMax" data-i18n-aria="winMax"><svg viewBox="0 0 10 10"><rect x="2.2" y="2.2" width="5.6" height="5.6" rx=".6"/></svg></button>
          <button class="tb-btn close" id="btnClose" data-i18n-aria="winClose"><svg viewBox="0 0 10 10"><path d="M2.4 2.4l5.2 5.2M7.6 2.4L2.4 7.6"/></svg></button>
        </div>
      </header>

      <div class="content" id="content">
        <div class="stack">
        <div class="ambient" aria-hidden="true"></div>

        <section class="glass hero">
          <div>
            <div class="hero-top">
              <span class="badge" id="themeBadge"><i class="dot"></i><span id="themeBadgeText">—</span></span>
              <div class="hero-toggle">
                <span data-i18n="autoSwitch">Auto switching</span>
                <label class="switch"><input type="checkbox" id="enabled"><span class="track"></span><span class="knob"></span></label>
              </div>
            </div>
            <div class="hero-line"><span class="k" id="cdLabel"></span><span class="big" id="cdValue">—</span></div>
            <div class="chips" id="heroChips"></div>
          </div>

          <div class="arc-wrap" id="arcWrap">
            <svg class="arc" viewBox="0 0 360 132" role="img" aria-label="sun arc">
              <defs>
                <linearGradient id="arcGrad" x1="0" y1="0" x2="1" y2="0">
                  <stop offset="0" stop-color="#FFB340"/><stop offset=".5" stop-color="#FFD166"/>
                  <stop offset="1" stop-color="#FF8A5C"/>
                </linearGradient>
                <linearGradient id="arcGradNight" x1="0" y1="0" x2="1" y2="0">
                  <stop offset="0" stop-color="#7C8BC7"/><stop offset=".5" stop-color="#B9C6F5"/>
                  <stop offset="1" stop-color="#6E7FB8"/>
                </linearGradient>
                <radialGradient id="haloGrad">
                  <stop offset="0" stop-color="#FFD166" stop-opacity=".55"/>
                  <stop offset="1" stop-color="#FFD166" stop-opacity="0"/>
                </radialGradient>
                <radialGradient id="haloNight">
                  <stop offset="0" stop-color="#C9D6FF" stop-opacity=".55"/>
                  <stop offset=".45" stop-color="#9FB2F0" stop-opacity=".22"/>
                  <stop offset="1" stop-color="#8FA3E8" stop-opacity="0"/>
                </radialGradient>
                <linearGradient id="moonGrad" x1="0" y1="0" x2="1" y2="1">
                  <stop offset="0" stop-color="#FFFFFF"/>
                  <stop offset=".55" stop-color="#EDF2FF"/>
                  <stop offset="1" stop-color="#C6D3F2"/>
                </linearGradient>
                <mask id="moonMask" maskUnits="userSpaceOnUse" x="-12" y="-12" width="24" height="24">
                  <circle cx="0" cy="0" r="8" fill="#fff"/>
                  <circle cx="4.3" cy="-2.4" r="6.9" fill="#000"/>
                </mask>
              </defs>
              <path class="track" d="M18 112 H342"/>
              <path class="track" d="M18 112 A 162 96 0 0 1 342 112"/>
              <path class="prog" id="arcProg" d="M18 112 A 162 96 0 0 1 342 112" stroke-dasharray="1000" stroke-dashoffset="1000"/>
              <circle class="halo" id="arcHalo" cx="18" cy="112" r="22"/>
              <circle class="sun" id="arcSun" cx="18" cy="112" r="5.5" fill="#FFD166"/>
              <g id="arcMoonG" class="moon" style="display:none">
                <circle cx="0" cy="0" r="30" fill="url(#haloNight)"/>
                <circle cx="0" cy="0" r="8" fill="url(#moonGrad)" mask="url(#moonMask)"/>
              </g>
            </svg>
            <div class="arc-label">
              <span><span id="arcLeftLabel">Sunrise</span> <b id="arcLeftTime">--:--</b></span>
              <span><span id="arcRightLabel">Sunset</span> <b id="arcRightTime">--:--</b></span>
            </div>
          </div>

          <div class="timeline" id="timeline">
            <div class="bar" id="tlBar"></div>
            <div class="now" id="tlNow" style="left:39%"></div>
            <div class="ticks"><span id="tlTick0">07:00</span><span>13:00</span><span id="tlTick1">19:00</span><span>01:00</span></div>
          </div>
        </section>

        <div class="cols">
          <section class="glass card">
            <div class="card-head">
              <span class="ico"><svg viewBox="0 0 24 24"><circle cx="12" cy="12" r="8.5"/><path d="M12 7.5V12l3 1.8"/></svg></span>
              <h2 data-i18n="schedule">Schedule</h2>
              <span class="hint" data-i18n="autoSave">Auto-saved</span>
            </div>
            <div class="seg" data-seg="mode" role="radiogroup" style="--n:2;--i:0">
              <span class="seg__thumb"></span>
              <button class="seg__btn is-on" role="radio" aria-checked="true" data-val="solar"><span data-i18n="modeSolarShort">Sunrise / sunset</span></button>
              <button class="seg__btn" role="radio" aria-checked="false" data-val="fixed"><span data-i18n="modeFixedShort">Fixed times</span></button>
            </div>

            <div class="pane is-on" data-pane="mode-solar" style="margin-top:12px">
              <div class="field"><span class="lab" data-i18n="sunriseToLight">Sunrise → light</span><span class="val" id="schRise">--:--</span></div>
              <div class="field"><span class="lab" data-i18n="sunsetToDark">Sunset → dark</span><span class="val" id="schSet">--:--</span></div>
              <div class="field"><span class="lab" data-i18n="todayDaylight">Daylight today</span><span class="val"><span class="muted" id="daylight">—</span></span></div>
              <div class="field"><span class="lab" data-i18n="currentPeriod">Current period</span><span class="val" id="curPeriod">—</span></div>
            </div>

            <div class="pane" data-pane="mode-fixed" style="margin-top:12px">
              <div class="field"><span class="lab" data-i18n="labelDarkFrom">Dark from</span><span class="val"><input type="time" id="darkStart"></span></div>
              <div class="field"><span class="lab" data-i18n="labelLightFrom">Light from</span><span class="val"><input type="time" id="lightStart"></span></div>
              <div class="field"><span class="lab" data-i18n="darkDuration">Dark hours</span><span class="val"><span class="muted" id="fixedDarkDur">—</span></span></div>
              <div class="field"><span class="lab" data-i18n="lightDuration">Light hours</span><span class="val"><span class="muted" id="fixedLightDur">—</span></span></div>
            </div>
          </section>

          <section class="glass card">
            <div class="card-head">
              <span class="ico"><svg viewBox="0 0 24 24"><path d="M12 21s6.5-5.6 6.5-10.5a6.5 6.5 0 1 0-13 0C5.5 15.4 12 21 12 21z"/><circle cx="12" cy="10.5" r="2.4"/></svg></span>
              <h2 data-i18n="location">Location</h2>
              <span class="hint" id="locHint"></span>
            </div>
            <div class="seg" data-seg="loc" role="radiogroup" style="--n:3;--i:0">
              <span class="seg__thumb"></span>
              <button class="seg__btn is-on" role="radio" aria-checked="true" data-val="auto"><span data-i18n="locAutoShort">Auto</span></button>
              <button class="seg__btn" role="radio" aria-checked="false" data-val="manual"><span data-i18n="locManualShort">Manual</span></button>
              <button class="seg__btn" role="radio" aria-checked="false" data-val="city"><span data-i18n="locCityShort">City</span></button>
            </div>

            <div class="pane is-on" data-pane="loc-auto" style="margin-top:12px">
              <div class="field"><span class="lab" data-i18n="locSource">Source</span><span class="val" id="locSourceVal">—</span></div>
              <div class="field"><span class="lab" data-i18n="statusCoords">Coordinates</span><span class="val" id="locCoordsVal">—</span></div>
              <div class="field" style="justify-content:flex-start">
                <button class="btn" id="detectBtn">
                  <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round">
                    <path d="M20 12a8 8 0 1 1-2.3-5.6"/><path d="M20 4v4h-4"/>
                  </svg>
                  <span id="detectBtnText" data-i18n="detectNow">Detect now</span>
                </button>
              </div>
              <div class="note info" id="noteInfo"><span class="spinner"></span><span id="noteInfoText"></span></div>
              <div class="note error" id="noteError"><span>⚠</span><span id="noteErrorText"></span></div>
            </div>

            <div class="pane" data-pane="loc-manual" style="margin-top:12px">
              <div class="field"><span class="lab" data-i18n="latitude">Latitude</span><span class="val"><input type="number" step="0.0001" id="lat" placeholder="0"></span></div>
              <div class="field"><span class="lab" data-i18n="longitude">Longitude</span><span class="val"><input type="number" step="0.0001" id="lon" placeholder="0"></span></div>
            </div>

            <div class="pane" data-pane="loc-city" style="margin-top:12px">
              <div class="combo" id="cityCombo">
                <input class="combo__input" id="cityInput" type="text" autocomplete="off"
                       role="combobox" aria-expanded="false" aria-controls="cityList"
                       data-i18n-placeholder="searchCity" placeholder="Search city…">
                <div class="combo__list" id="cityList" role="listbox"></div>
              </div>
              <div class="field" style="margin-top:8px">
                <span class="lab" data-i18n="selectedCity">Selected</span>
                <span class="val" id="cityPicked">—</span>
              </div>
            </div>
          </section>
        </div>

        <section class="glass card">
          <div class="card-head">
            <span class="ico"><svg viewBox="0 0 24 24"><path d="M4 7h10M18 7h2M4 17h4M12 17h8"/><circle cx="16" cy="7" r="2.2"/><circle cx="10" cy="17" r="2.2"/></svg></span>
            <h2 data-i18n="general">General</h2>
            <span class="hint" data-i18n="autoSave">Auto-saved</span>
          </div>

          <div class="field" style="border-top:0;padding-top:0">
            <span class="lab" data-i18n="switchNow">Switch now</span>
            <span class="val">
              <span class="seg" data-seg="theme" role="radiogroup" style="--n:3;--i:0;min-width:246px">
                <span class="seg__thumb"></span>
                <button class="seg__btn is-on" role="radio" aria-checked="true" data-val="auto"><span data-i18n="themeAuto">Automatic</span></button>
                <button class="seg__btn" role="radio" aria-checked="false" data-val="dark"><span data-i18n="themeDark">Dark</span></button>
                <button class="seg__btn" role="radio" aria-checked="false" data-val="light"><span data-i18n="themeLight">Light</span></button>
              </span>
            </span>
          </div>

          <div class="field">
            <span class="lab" data-i18n="language">Language</span>
            <span class="val">
              <div class="select" data-select="lang">
                <button class="select__btn" aria-haspopup="listbox" aria-expanded="false">
                  <span class="select__label" data-i18n="followSystem">Follow system</span>
                  <svg viewBox="0 0 24 24"><path d="M6 9l6 6 6-6"/></svg>
                </button>
                <div class="select__menu" role="listbox">
                  <div class="select__item is-on" role="option" aria-selected="true" data-val=""><span data-i18n="followSystem">Follow system</span></div>
                  <div class="select__item" role="option" aria-selected="false" data-val="en">English</div>
                  <div class="select__item" role="option" aria-selected="false" data-val="zh">中文</div>
                </div>
              </div>
            </span>
          </div>

          <div class="field">
            <span class="lab" data-i18n="launchAtLogin">Launch at login</span>
            <span class="val"><label class="switch"><input type="checkbox" id="autoStart"><span class="track"></span><span class="knob"></span></label></span>
          </div>

          <div class="field">
            <span class="lab" data-i18n="closeToTray">Minimize to tray on close</span>
            <span class="val"><label class="switch"><input type="checkbox" id="closeTray"><span class="track"></span><span class="knob"></span></label></span>
          </div>

          <div class="field">
            <span class="lab" data-i18n="autoUpdateCheck">Check for updates automatically</span>
            <span class="val"><label class="switch"><input type="checkbox" id="autoUpdateCheck"><span class="track"></span><span class="knob"></span></label></span>
          </div>
        </section>

        <section class="glass card">
          <div class="card-head">
            <span class="ico"><svg viewBox="0 0 24 24"><circle cx="12" cy="12" r="8.5"/><path d="M12 11v5.5M12 7.8v.4"/></svg></span>
            <h2 data-i18n="about">About</h2>
          </div>
          <div class="about-row">
            <span class="k" data-i18n="version">Version</span>
            <span class="v" id="aboutVersion">—</span>
          </div>
          <div class="about-row">
            <span class="k" data-i18n="updateTitle">Updates</span>
            <span class="about-links">
              <span class="muted" id="updateStatus">—</span>
              <button class="btn small" id="btnUpdate" data-i18n="checkUpdates">Check for updates</button>
            </span>
          </div>
          <div class="update-progress" id="updateProgress" hidden><i id="updateProgressFill"></i></div>
          <div class="about-row">
            <span class="k" data-i18n="repoLinks">Repository &amp; updates</span>
            <span class="about-links">
              <button class="btn small" id="btnRepo" data-i18n="openRepo">GitHub repository</button>
              <button class="btn small" id="btnReleases" data-i18n="openReleases">Release page</button>
              <button class="btn small" id="btnLogs" data-i18n="openLogs">Open log folder</button>
            </span>
          </div>
          <div class="about-row">
            <span class="k" data-i18n="diagnostics">Diagnostic log</span>
            <span class="v about-path">%APPDATA%\\AutoDarkMode\\app.log</span>
          </div>
        </section>

        <footer class="foot" id="appVersion"></footer>
        </div>
      </div>
    </div>

    <div class="toast" id="toast" role="status" aria-live="polite">
      <svg viewBox="0 0 24 24"><path d="M4 12.5l5 5L20 6.5"/></svg>
      <span id="toastText">Saved</span>
    </div>`;
}

// --- boot ----------------------------------------------------------------

async function init() {
    const root = $('#app');
    root.innerHTML = template();

    // Version (footer + about)
    try {
        const v = await GetVersion();
        if (v) {
            $('#appVersion').textContent = 'v' + v;
            $('#aboutVersion').textContent = 'v' + v;
        }
    } catch (e) {
        console.error('GetVersion failed', e);
    }

    try {
        cities = (await GetCities()) || [];
    } catch (e) {
        cities = [];
    }

    try {
        cfg = await GetConfig();
    } catch (e) {
        console.error('GetConfig failed', e);
    }

    // Segments
    setupSeg('mode', (v) => {
        showPane('mode', v);
        if (cfg) cfg.mode = v;
        scheduleSave();
    });
    setupSeg('loc', (v) => {
        showPane('loc', v);
        if (cfg) cfg.locationSource = v;
        // Do not persist a half-configured source: saving "city" before a city
        // is picked (or "manual" with empty fields) made the schedule fall
        // back to an error state and the sunrise/sunset arc went blank.
        if (v === 'city' && !$('#cityPicked').dataset.city) return;
        if (v === 'manual' && (!$('#lat').value || !$('#lon').value)) return;
        scheduleSave();
    });
    setupSeg('theme', (v) => {
        if (cfg) cfg.manualTheme = v;
        SetManualTheme(v);
        toast(t('saved'));
    });

    setupSelect('lang', (v) => {
        if (cfg) cfg.lang = v;
        applyLanguage(v || (status ? status.lang : 'en'));
        SetLang(v);
    });

    renderForm();
    applyLanguage((cfg && cfg.lang) || 'en');
    renderCityList('');
    openCombo(false);
    requestAnimationFrame(layoutAllSegs);

    await refresh();

    // Live state pushed from the backend (scheduler / apply / language).
    EventsOn('state', (s) => {
        status = s;
        renderStatus();
    });

    // --- controls ---------------------------------------------------------
    $('#enabled').addEventListener('change', () => {
        if (cfg) cfg.enabled = $('#enabled').checked;
        SetEnabled($('#enabled').checked);
    });
    $('#autoStart').addEventListener('change', () => {
        if (cfg) cfg.autoStart = $('#autoStart').checked;
        SetAutoStart($('#autoStart').checked);
        toast(t('saved'));
    });
    $('#closeTray').addEventListener('change', () => {
        if (cfg) cfg.closeToTray = $('#closeTray').checked;
        SetCloseTray($('#closeTray').checked);
        toast(t('saved'));
    });
    if ($('#autoUpdateCheck')) {
        $('#autoUpdateCheck').addEventListener('change', () => {
            if (cfg) cfg.autoUpdateCheck = $('#autoUpdateCheck').checked;
            scheduleSave();
            toast(t('saved'));
        });
    }
    ['#darkStart', '#lightStart', '#lat', '#lon'].forEach((sel) => {
        $(sel).addEventListener('change', scheduleSave);
    });

    // City combobox
    const cityInput = $('#cityInput');
    cityInput.addEventListener('focus', () => { renderCityList(cityInput.value); openCombo(true); });
    cityInput.addEventListener('input', () => { renderCityList(cityInput.value); openCombo(true); });
    cityInput.addEventListener('keydown', (e) => {
        if (e.key === 'ArrowDown') { e.preventDefault(); activeCity = Math.min(activeCity + 1, filteredCities.length - 1); highlightCity(); }
        if (e.key === 'ArrowUp') { e.preventDefault(); activeCity = Math.max(activeCity - 1, 0); highlightCity(); }
        if (e.key === 'Enter') { e.preventDefault(); pickCity(activeCity); }
        if (e.key === 'Escape') { openCombo(false); cityInput.blur(); }
    });
    $('#cityList').addEventListener('mousedown', (e) => {
        const opt = e.target.closest('.combo__opt');
        if (opt) { e.preventDefault(); pickCity(Number(opt.dataset.i)); }
    });
    document.addEventListener('mousedown', (e) => {
        if (!e.target.closest('#cityCombo')) openCombo(false);
        if (!e.target.closest('[data-select="lang"]')) closeSelect('lang');
    });

    // Detect button
    $('#detectBtn').addEventListener('click', async () => {
        const btn = $('#detectBtn');
        const label = $('#detectBtnText');
        btn.disabled = true;
        label.textContent = t('detecting');
        detecting = true;
        clearLocatingNote();
        try {
            const r = await DetectLocation();
            if (r.error) {
                $('#noteErrorText').textContent = r.error;
                $('#noteError').classList.add('is-on');
            } else {
                $('#noteError').classList.remove('is-on');
                clearLocatingNote();
                toast(t('saved'));
            }
        } catch (e) {
            $('#noteErrorText').textContent = String(e);
            $('#noteError').classList.add('is-on');
        } finally {
            detecting = false;
            btn.disabled = false;
            label.textContent = t('detectNow');
            await refresh();
        }
    });

    // Location progress events
    EventsOn('locating', (p) => {
        if (!detecting || !p || !p.stage) return;
        const key = 'locating' + cap(p.stage);
        setLocatingNote((key in I18N[lang]) ? t(key) : t('locatingPermission'));
    });

    // About
    $('#btnRepo').addEventListener('click', () => OpenRepository());
    $('#btnReleases').addEventListener('click', () => OpenReleases());
    $('#btnLogs').addEventListener('click', () => OpenLogsFolder());

    // --- updater ----------------------------------------------------------
    $('#btnUpdate').addEventListener('click', async () => {
        const state = updateInfo ? updateInfo.state : 'idle';
        if (state === 'available') {
            await DownloadUpdate();
        } else if (state === 'ready') {
            await InstallUpdate();
        } else {
            const btn = $('#btnUpdate');
            btn.disabled = true;
            try {
                const info = await CheckForUpdate();
                renderUpdate(info);
            } catch (e) {
                console.error('CheckForUpdate failed', e);
            } finally {
                btn.disabled = false;
            }
        }
    });
    EventsOn('update:changed', (info) => renderUpdate(info));

    // --- window controls --------------------------------------------------
    $('#btnMin').addEventListener('click', () => WindowMinimise());
    $('#btnMax').addEventListener('click', () => { WindowToggleMaximise(); setTimeout(updateMaxIcon, 150); });
    $('#btnClose').addEventListener('click', () => {
        if (cfg && cfg.closeToTray) WindowHide();
        else Quit();
    });
    $('#tbDrag').addEventListener('dblclick', () => { WindowToggleMaximise(); setTimeout(updateMaxIcon, 150); });
    window.addEventListener('resize', updateMaxIcon);
    window.addEventListener('resize', layoutAllSegs);
    updateMaxIcon();

    // --- pointer-following specular highlight -----------------------------
    const glassEls = $$('.glass');
    let raf = 0, px = 0, py = 0;
    window.addEventListener('pointermove', (e) => {
        px = e.clientX; py = e.clientY;
        if (raf) return;
        raf = requestAnimationFrame(() => {
            raf = 0;
            for (const g of glassEls) {
                const r = g.getBoundingClientRect();
                if (px < r.left - 60 || px > r.right + 60 || py < r.top - 60 || py > r.bottom + 60) continue;
                g.style.setProperty('--gx', ((px - r.left) / r.width * 100).toFixed(1) + '%');
                g.style.setProperty('--gy', ((py - r.top) / r.height * 100).toFixed(1) + '%');
            }
        });
    }, { passive: true });

    // Keep the countdown honest without waiting for a state push.
    setInterval(() => { if (status) renderHero(); }, 30000);

    // Inter is a webfont: re-measure the pills once it has actually loaded.
    if (document.fonts && document.fonts.ready) {
        document.fonts.ready.then(layoutAllSegs);
    }
}

init();
