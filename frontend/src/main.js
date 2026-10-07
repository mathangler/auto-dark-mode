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
} from '../wailsjs/go/main/App';
import {EventsOn} from '../wailsjs/runtime/runtime';

// --- i18n ----------------------------------------------------------------

const I18N = {
    en: {
        statusCurrent: 'Current',
        statusNext: 'Next change',
        statusSource: 'Source',
        statusCoords: 'Coordinates',
        schedule: 'Schedule',
        modeSolar: 'Sunrise / sunset (location-based)',
        modeFixed: 'Fixed times',
        labelDarkFrom: 'Dark from',
        labelLightFrom: 'Light from',
        sunrise: 'Sunrise',
        sunset: 'Sunset',
        location: 'Location',
        locAuto: 'Auto-detect (GPS → IP)',
        locManual: 'Enter coordinates',
        locCity: 'Pick a city',
        fieldLatitude: 'Latitude',
        fieldLongitude: 'Longitude',
        detectNow: 'Detect location now',
        detecting: 'Locating…',
        locatingPermission: 'Requesting location permission…',
        locatingGetting: 'Getting your location…',
        locatingIpDenied: 'You didn’t grant location permission — locating via IP…',
        locatingIpFallback: 'Unable to get your location — locating via IP…',
        manualOverride: 'Manual override',
        themeAuto: 'Automatic',
        themeDark: 'Dark now',
        themeLight: 'Light now',
        launchAtLogin: 'Launch at login',
        closeToTray: 'Minimize to tray on close',
        language: 'Language',
        followSystem: 'Follow system',
        searchCity: 'Search city…',
        save: 'Save settings',
        settingsGroup: 'Schedule & location',
        saveSchedule: 'Save schedule & location',
        saving: 'Saving…',
        saveNote: 'Schedule and location changes apply when you save them.',
        immediate: 'Applies immediately',
        saved: 'Saved ✓',
        srcAuto: 'Auto (GPS → IP)',
        srcManual: 'Manual',
        srcCity: 'City',
        srcGps: 'GPS',
        srcIp: 'IP',
    },
    zh: {
        statusCurrent: '当前',
        statusNext: '下次切换',
        statusSource: '位置来源',
        statusCoords: '坐标',
        schedule: '切换计划',
        modeSolar: '按日出 / 日落（根据位置）',
        modeFixed: '固定时间',
        labelDarkFrom: '深色从',
        labelLightFrom: '浅色从',
        sunrise: '日出',
        sunset: '日落',
        location: '位置',
        locAuto: '自动定位（GPS → IP）',
        locManual: '手动输入坐标',
        locCity: '选择城市',
        fieldLatitude: '纬度',
        fieldLongitude: '经度',
        detectNow: '立即探测位置',
        detecting: '定位中…',
        locatingPermission: '正在申请定位权限…',
        locatingGetting: '正在获取位置…',
        locatingIpDenied: '未授予定位权限，尝试通过 IP 确定位置…',
        locatingIpFallback: '无法获取当前位置，尝试通过 IP 确定位置…',
        manualOverride: '手动切换',
        themeAuto: '自动',
        themeDark: '立即深色',
        themeLight: '立即浅色',
        launchAtLogin: '开机自启动',
        closeToTray: '关闭时最小化到托盘',
        language: '语言',
        followSystem: '跟随系统',
        searchCity: '搜索城市…',
        save: '保存设置',
        settingsGroup: '计划与位置设置',
        saveSchedule: '保存计划与位置设置',
        saving: '保存中…',
        saveNote: '计划与位置设置修改后点击“保存”生效',
        immediate: '立即生效',
        saved: '已保存 ✓',
        srcAuto: '自动定位（GPS→IP）',
        srcManual: '手动',
        srcCity: '城市',
        srcGps: 'GPS',
        srcIp: 'IP',
    },
};

let lang = 'en';
let cfg = null;
let cities = [];
let status = null;
let manualTheme = 'auto'; // in-form override selection
let detecting = false;    // true while a location detection is running

const t = (key) => (I18N[lang] && I18N[lang][key]) || I18N.en[key] || key;

// --- Small helpers -------------------------------------------------------

const $ = (sel) => document.querySelector(sel);
const el = (tag, cls, text) => {
    const n = document.createElement(tag);
    if (cls) n.className = cls;
    if (text !== undefined) n.textContent = text;
    return n;
};

// --- Theme + language application ---------------------------------------

function applyTheme(theme) {
    document.documentElement.dataset.theme = theme === 'light' ? 'light' : 'dark';
}

// Follow the OS theme in real time via the browser's color-scheme media query,
// so the window repaints instantly on switch (no dependency on a Go event).
const lightPref = window.matchMedia('(prefers-color-scheme: light)');
if (lightPref.addEventListener) {
    lightPref.addEventListener('change', () => applyTheme(lightPref.matches ? 'light' : 'dark'));
}
applyTheme(lightPref.matches ? 'light' : 'dark');

function applyLanguage(effective) {
    if (effective && I18N[effective]) lang = effective;
    document.querySelectorAll('[data-i18n]').forEach((node) => {
        node.textContent = t(node.dataset.i18n);
    });
    document.querySelectorAll('option[data-i18n-opt]').forEach((opt) => {
        opt.textContent = t(opt.dataset.i18nOpt);
    });
    document.querySelectorAll('[data-i18n-placeholder]').forEach((node) => {
        node.setAttribute('placeholder', t(node.dataset.i18nPlaceholder));
    });
    const det = $('#detectBtn');
    if (det) det.textContent = t('detectNow');

    // City names differ per language — re-render, keeping the selection.
    const citySel = $('#city');
    if (citySel) {
        const keep = citySel.value;
        const filterEl = $('#cityFilter');
        populateCities(filterEl ? filterEl.value : '');
        if (keep && Array.from(citySel.options).some((o) => o.value === keep)) {
            citySel.value = keep;
        }
    }
}

function renderStatus() {
    if (!status) return;
    // keep the window following the OS theme and language
    applyTheme(status.currentTheme);
    if (status.lang && status.lang !== lang) applyLanguage(status.lang);

    // Reflect backend-driven changes (e.g. actions taken from the tray menu).
    if (status.manualTheme && status.manualTheme !== manualTheme) {
        manualTheme = status.manualTheme;
        setThemeButtons(status.manualTheme);
    }
    if (status.enabled !== undefined) $('#enabled').checked = !!status.enabled;

    const theme = status.currentTheme || '—';
    $('#st-theme').textContent = theme;
    $('#st-theme').className = 'badge ' + (theme === 'dark' ? 'dark' : 'light');
    $('#st-next').textContent = status.nextTransition || '—';
    $('#st-sunrise').textContent = status.sunrise || '--:--';
    $('#st-sunset').textContent = status.sunset || '--:--';

    const loc = status.location || {};
    $('#st-source').textContent = loc.source ? t('src' + cap(loc.source)) : '—';
    const locName = loc.name ? ' (' + localizedCityName(loc) + ')' : '';
    $('#st-coords').textContent = loc.error
        ? loc.error
        : (loc.lat !== undefined && loc.lon !== undefined
            ? `${fmtCoord(loc.lat, 'lat')}, ${fmtCoord(loc.lon, 'lon')}${locName}`
            : '—');

    const err = status.lastError || '';
    const errEl = $('#st-error');
    errEl.textContent = err;
    errEl.style.display = err ? 'block' : 'none';
}

function cap(s) { return s.charAt(0).toUpperCase() + s.slice(1); }

function fmtCoord(v, kind) {
    if (v === undefined || isNaN(v)) return '?';
    const d = Math.abs(v);
    const dir = kind === 'lat' ? (v >= 0 ? 'N' : 'S') : (v >= 0 ? 'E' : 'W');
    return d.toFixed(3) + '°' + dir;
}

// --- Form rendering from config -----------------------------------------

function renderForm() {
    $('#enabled').checked = !!cfg.enabled;
    $('#autoStart').checked = !!cfg.autoStart;
    $('#closeTray').checked = cfg.closeToTray === true;
    $('#lang').value = cfg.lang || '';

    $('#modeSolar').checked = cfg.mode !== 'fixed';
    $('#modeFixed').checked = cfg.mode === 'fixed';
    $('#darkStart').value = cfg.darkStart || '19:00';
    $('#lightStart').value = cfg.lightStart || '07:00';

    const src = cfg.locationSource || 'auto';
    $('#locAuto').checked = src === 'auto';
    $('#locManual').checked = src === 'manual';
    $('#locCity').checked = src === 'city';
    // Leave the fields empty unless the user has previously stored a value;
    // an empty field saves as 0 (default 0,0).
    $('#lat').value = (cfg.lat && cfg.lat !== 0) ? cfg.lat : '';
    $('#lon').value = (cfg.lon && cfg.lon !== 0) ? cfg.lon : '';
    $('#city').value = cfg.city || '';

    manualTheme = cfg.manualTheme || 'auto';
    setThemeButtons(manualTheme);
    updateVisibility();
}

function setThemeButtons(mode) {
    for (const b of document.querySelectorAll('.themebtn')) b.classList.remove('active');
    const map = {auto: '#tAuto', dark: '#tDark', light: '#tLight'};
    const target = document.querySelector(map[mode] || map.auto);
    if (target) target.classList.add('active');
}

function updateVisibility() {
    const fixed = $('#modeFixed').checked;
    $('#fixedTimes').style.display = fixed ? 'block' : 'none';
    $('#solarInfo').style.display = fixed ? 'none' : 'block';

    const src = selectedLocationSource();
    $('#manualBox').style.display = src === 'manual' ? 'block' : 'none';
    $('#cityBox').style.display = src === 'city' ? 'block' : 'none';
    $('#detectBtn').style.display = src === 'auto' ? 'inline-block' : 'none';
}

function selectedLocationSource() {
    if ($('#locManual').checked) return 'manual';
    if ($('#locCity').checked) return 'city';
    return 'auto';
}

// populateCities fills the city <select> with entries matching the filter.
// Options display localized names (Chinese in zh, English otherwise) while
// their values stay the canonical English key used for storage.
function populateCities(filter) {
    const sel = $('#city');
    if (!sel) return;
    const q = (filter || '').trim();
    const ql = q.toLowerCase();
    sel.innerHTML = '';
    for (const c of cities) {
        const nameZh = c.nameZh || '';
        const match = c.name.toLowerCase().includes(ql) || nameZh.includes(q);
        if (q && !match) continue;
        const o = el('option', '', lang === 'zh' && nameZh ? nameZh : c.name);
        o.value = c.name;
        sel.appendChild(o);
    }
}

// localizedCityName returns a city's name in the current UI language.
function localizedCityName(loc) {
    if (lang === 'zh' && loc.nameZh) return loc.nameZh;
    return loc.name;
}

// setLocMsg shows a message in the detection feedback row (with a close ×).
function setLocMsg(text) {
    const box = $('#locMsg');
    const txt = $('#locMsgText');
    if (!box || !txt || !text) { clearLocMsg(); return; }
    txt.textContent = text;
    box.style.display = 'flex';
}

// clearLocMsg hides the detection feedback row.
function clearLocMsg() {
    const box = $('#locMsg');
    if (box) box.style.display = 'none';
}

// --- Build config from the form -----------------------------------------

function readConfig() {
    const locSrc = selectedLocationSource();
    const c = {
        enabled: $('#enabled').checked,
        autoStart: $('#autoStart').checked,
        closeToTray: $('#closeTray').checked,
        lang: $('#lang').value || '',
        mode: $('#modeFixed').checked ? 'fixed' : 'solar',
        darkStart: $('#darkStart').value || '19:00',
        lightStart: $('#lightStart').value || '07:00',
        locationSource: locSrc,
        manualTheme,
    };
    if (locSrc === 'manual') {
        const lat = parseFloat($('#lat').value);
        const lon = parseFloat($('#lon').value);
        c.lat = isNaN(lat) ? 0 : lat;
        c.lon = isNaN(lon) ? 0 : lon;
        c.city = '';
    } else {
        c.lat = 0;
        c.lon = 0;
        c.city = locSrc === 'city' ? $('#city').value : '';
    }
    return c;
}

async function save() {
    cfg = readConfig();
    const btn = $('#saveBtn');
    if (btn) {
        btn.disabled = true;
        btn.textContent = t('saving');
    }
    try {
        await SaveConfig(cfg);
        if (btn) btn.disabled = false;
        showSavedConfirm();
    } catch (e) {
        console.error('SaveConfig failed', e);
        if (btn) {
            btn.disabled = false;
            btn.textContent = t('saveSchedule');
        }
    }
    await refresh();
}

// showSavedConfirm briefly flashes "Saved ✓" on the save button so it's clear
// the schedule & location settings were committed.
function showSavedConfirm() {
    const b = $('#saveBtn');
    if (!b) return;
    b.textContent = t('saved');
    clearTimeout(window.__saveTimer);
    window.__saveTimer = setTimeout(() => { b.textContent = t('saveSchedule'); }, 1600);
}

async function refresh() {
    try {
        status = await GetState();
        renderStatus();
    } catch (e) {
        console.error('GetState failed', e);
    }
}

// --- Boot ----------------------------------------------------------------

async function init() {
    const root = $('#app');
    root.innerHTML = `
      <div class="card">
        <div class="head">
          <h1>Auto Dark Mode</h1>
          <label class="switch" title="Enable automatic switching">
            <input type="checkbox" id="enabled">
            <span class="slider"></span>
          </label>
        </div>

        <div class="status" id="statusTop">
          <div class="stat"><span class="lbl" data-i18n="statusCurrent">Current</span><span class="badge light" id="st-theme">—</span></div>
          <div class="stat"><span class="lbl" data-i18n="statusNext">Next change</span><span id="st-next">—</span></div>
          <div class="stat"><span class="lbl" data-i18n="statusSource">Source</span><span id="st-source">—</span></div>
          <div class="stat"><span class="lbl" data-i18n="statusCoords">Coordinates</span><span id="st-coords">—</span></div>
        </div>
        <div class="error-box" id="st-error" style="display:none"></div>

        <section class="panel settings-block">
          <h2 data-i18n="settingsGroup">Schedule &amp; location</h2>
          <div class="grid">
          <section class="panel">
            <h2 data-i18n="schedule">Schedule</h2>
            <label class="radio"><input type="radio" name="mode" id="modeSolar" value="solar"><span data-i18n="modeSolar">Sunrise / sunset (location-based)</span></label>
            <label class="radio"><input type="radio" name="mode" id="modeFixed" value="fixed"><span data-i18n="modeFixed">Fixed times</span></label>
            <div id="fixedTimes" style="display:none">
              <div class="row"><label class="mini" for="darkStart" data-i18n="labelDarkFrom">Dark from</label><input type="time" id="darkStart"></div>
              <div class="row"><label class="mini" for="lightStart" data-i18n="labelLightFrom">Light from</label><input type="time" id="lightStart"></div>
            </div>
            <div id="solarInfo" class="solar-row">
              <span data-i18n="sunrise">Sunrise</span> <b id="st-sunrise">--:--</b>
              <span data-i18n="sunset">Sunset</span> <b id="st-sunset">--:--</b>
            </div>
          </section>

          <section class="panel">
            <h2 data-i18n="location">Location</h2>
            <label class="radio"><input type="radio" name="loc" id="locAuto" value="auto"><span data-i18n="locAuto">Auto-detect (GPS → IP)</span></label>
            <label class="radio"><input type="radio" name="loc" id="locManual" value="manual"><span data-i18n="locManual">Enter coordinates</span></label>
            <label class="radio"><input type="radio" name="loc" id="locCity" value="city"><span data-i18n="locCity">Pick a city</span></label>
            <div id="manualBox" style="display:none">
              <div class="row"><label class="mini" for="lat" data-i18n="fieldLatitude">Latitude</label><input type="number" step="0.0001" id="lat" placeholder="0"></div>
              <div class="row"><label class="mini" for="lon" data-i18n="fieldLongitude">Longitude</label><input type="number" step="0.0001" id="lon" placeholder="0"></div>
            </div>
            <div id="cityBox" style="display:none">
              <input type="text" id="cityFilter" class="city-filter" data-i18n-placeholder="searchCity"/>
              <select id="city"></select>
            </div>
            <button id="detectBtn" style="display:none" class="btn ghost" data-i18n="detectNow">Detect location now</button>
            <div id="locMsg" class="loc-msg" style="display:none">
              <span id="locMsgText"></span>
              <button type="button" id="locMsgClose" class="loc-close" aria-label="Close">×</button>
            </div>
          </section>
        </div>

        <div class="save-bar">
          <span class="save-note" data-i18n="saveNote">Schedule and location changes apply when you save them.</span>
          <button id="saveBtn" class="btn primary" data-i18n="saveSchedule">Save schedule &amp; location</button>
        </div>
        </section>

        <section class="panel actions">
          <h2 data-i18n="immediate">Applies immediately</h2>
          <div class="btn-group">
            <button id="tAuto"   class="btn themebtn" data-i18n="themeAuto">Automatic</button>
            <button id="tDark"   class="btn themebtn" data-i18n="themeDark">Dark now</button>
            <button id="tLight"  class="btn themebtn" data-i18n="themeLight">Light now</button>
          </div>

          <div class="row">
            <label class="mini" for="lang" data-i18n="language">Language</label>
            <select id="lang">
              <option value="" data-i18n-opt="followSystem">Follow system</option>
              <option value="en">English</option>
              <option value="zh">中文</option>
            </select>
          </div>

          <div class="switch-col">
            <label class="switch-line"><input type="checkbox" id="autoStart"><span data-i18n="launchAtLogin">Launch at login</span></label>
            <label class="switch-line"><input type="checkbox" id="closeTray"><span data-i18n="closeToTray">Minimize to tray on close</span></label>
          </div>
        </section>

        <div class="card-foot" id="appVersion"></div>
      </div>
    `;

    // version footer (bound const; rules: docs/VERSIONING.md)
    try {
        const v = await GetVersion();
        if (v) $('#appVersion').textContent = 'v' + v;
    } catch (e) {
        console.error('GetVersion failed', e);
    }

    // populate city dropdown
    try {
        cities = await GetCities() || [];
    } catch (e) {
        cities = [];
    }
    populateCities('');

    try {
        cfg = await GetConfig();
    } catch (e) {
        console.error('GetConfig failed', e);
    }
    renderForm();
    applyLanguage(status ? status.lang : (cfg.lang || 'en'));
    await refresh();

    // Live updates pushed from the backend (scheduler / apply / language).
    EventsOn('state', (s) => {
        status = s;
        renderStatus();
    });

    // Events
    $('#enabled').addEventListener('change', () => {
        cfg.enabled = $('#enabled').checked;
        SetEnabled(cfg.enabled);
    });
    // Events — schedule & location are saved by the Save button below;
    // the remaining controls apply immediately on their own.
    for (const r of document.querySelectorAll('input[name=mode]')) r.addEventListener('change', updateVisibility);
    for (const r of document.querySelectorAll('input[name=loc]')) r.addEventListener('change', updateVisibility);
    $('#lang').addEventListener('change', () => {
        const v = $('#lang').value || '';
        cfg.lang = v;
        if (v === 'zh' || v === 'en') applyLanguage(v);
        SetLang(v);
    });
    $('#closeTray').addEventListener('change', () => { SetCloseTray($('#closeTray').checked); });
    $('#autoStart').addEventListener('change', () => { SetAutoStart($('#autoStart').checked); });
    $('#saveBtn').addEventListener('click', save);
    $('#detectBtn').addEventListener('click', async () => {
        const btn = $('#detectBtn');
        btn.disabled = true;
        btn.textContent = t('detecting');
        detecting = true;
        clearLocMsg();
        try {
            const r = await DetectLocation();
            if (r.error) {
                setLocMsg(r.error);
            } else {
                clearLocMsg();
                $('#st-coords').textContent =
                    `${fmtCoord(r.lat, 'lat')}, ${fmtCoord(r.lon, 'lon')} via ${t('src' + cap(r.source))}`;
            }
        } catch (e) {
            setLocMsg(String(e));
        } finally {
            detecting = false;
            btn.disabled = false;
            btn.textContent = t('detectNow');
        }
    });
    $('#locMsgClose').addEventListener('click', clearLocMsg);
    // Live progress from the backend while detecting a location. Events can
    // be delivered after the call resolves, so ignore them once detection ends.
    EventsOn('locating', (p) => {
        if (!detecting || !p || !p.stage) return;
        const key = 'locating' + cap(p.stage);
        setLocMsg((key in I18N[lang]) ? t(key) : t('locatingPermission'));
    });
    $('#cityFilter').addEventListener('input', (e) => {
        const prev = $('#city').value;
        populateCities(e.target.value);
        if (prev && Array.from($('#city').options).some((o) => o.value === prev)) {
            $('#city').value = prev;
        } else {
            $('#city').selectedIndex = 0;
        }
    });
    $('#tAuto').addEventListener('click', () => { manualTheme = 'auto'; setThemeButtons('auto'); SetManualTheme('auto'); });
    $('#tDark').addEventListener('click', () => { manualTheme = 'dark'; setThemeButtons('dark'); SetManualTheme('dark'); });
    $('#tLight').addEventListener('click', () => { manualTheme = 'light'; setThemeButtons('light'); SetManualTheme('light'); });
}

window.addEventListener('DOMContentLoaded', init);
