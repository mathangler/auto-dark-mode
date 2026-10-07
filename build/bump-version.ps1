<#
bump-version.ps1 — 版本升位 + 全量同步
本脚本即 docs/VERSIONING.md 规则的可执行形式。

用法:
    build\bump-version.ps1 -Patch    # 修复 / 加固 / 构建保底 +1   -> x.y.(z+1)
    build\bump-version.ps1 -Minor    # 新增功能或行为机制           -> x.(y+1).0
    build\bump-version.ps1 -Major    # 破坏性变更                  -> (x+1).0.0

规则摘要（详见 docs/VERSIONING.md）:
  * 每次产出新 exe 之前必须执行（至少 -Patch）；纯文档改动不执行本脚本。
  * 执行前先向用户报告"建议版本 + 理由"并获得确认，确认后才运行。
  * 升位后随执行在 docs/VERSIONING.md 的版本历史表追加一行。
  * 同步位置（真源 = version.go 的 const Version）:
      version.go                                    Go/UI/日志常量
      wails.json                                    info.productVersion -> NSIS DisplayVersion（安装包/控制面板）
      frontend/package.json                         前端包版本（一致性）
      docs/DESIGN.md                                文档标称版本
      build/windows/installer/wails_tools.nsh       NSIS 兜底值（wails 生成文件，缺失仅告警）
      build/windows/versioninfo.json + 仓库根 rsrc_versioninfo.syso
                                                    exe「详细信息」版本资源，由 build\make-syso.ps1 随升位再生
                                                    （wails 自带 info.json 嵌入路径实测无效，见 DESIGN §7）
#>
[CmdletBinding()]
param(
    [switch]$Patch,
    [switch]$Minor,
    [switch]$Major
)

$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot   # build\ -> 仓库根

$chosen = @()
if ($Patch) { $chosen += 'Patch' }
if ($Minor) { $chosen += 'Minor' }
if ($Major) { $chosen += 'Major' }
if ($chosen.Count -ne 1) {
    Write-Error "Exactly one of -Patch / -Minor / -Major must be given. Rules: docs/VERSIONING.md"
    exit 1
}

# --- read current version (source of truth: version.go) ---
$goPath = Join-Path $root 'version.go'
if (-not (Test-Path -LiteralPath $goPath)) {
    Write-Error "version.go not found at $goPath"
    exit 1
}
$goText = [System.IO.File]::ReadAllText($goPath)
if ($goText -notmatch 'const Version = "(\d+)\.(\d+)\.(\d+)"') {
    Write-Error "Cannot parse `"const Version = `"x.y.z`"`" in $goPath"
    exit 1
}
# NOTE: locals must NOT be named $major/$minor/$patch — PowerShell variable
# names are case-insensitive, so those alias the [switch]$Major/$Minor/$Patch
# parameters, and assigning an Int32 to a [switch]-constrained variable throws
# "cannot convert Int32 to SwitchParameter". Hence the ver* prefix.
$verMajor = [int]$Matches[1]
$verMinor = [int]$Matches[2]
$verPatch = [int]$Matches[3]
$old = "$verMajor.$verMinor.$verPatch"

switch ($chosen[0]) {
    'Major' { $verMajor++; $verMinor = 0; $verPatch = 0 }
    'Minor' { $verMinor++; $verPatch = 0 }
    'Patch' { $verPatch++ }
}
$new = "$verMajor.$verMinor.$verPatch"

# --- sync helper: UTF-8, preserves the original BOM state ---
# NSIS .nsh files MUST keep their UTF-8 BOM (makensis parses UTF-8 only with it).
function Update-File {
    param([string]$Path, [string]$Pattern, [string]$Replacement, [switch]$Soft)
    if (-not (Test-Path -LiteralPath $Path)) {
        if ($Soft) { Write-Warning "missing (soft, skipped): $Path"; return }
        Write-Error "missing required file: $Path"
        exit 1
    }
    $bytes = [System.IO.File]::ReadAllBytes($Path)
    $hasBom = $bytes.Length -ge 3 -and $bytes[0] -eq 0xEF -and $bytes[1] -eq 0xBB -and $bytes[2] -eq 0xBF
    $text = [System.IO.File]::ReadAllText($Path)
    $updated = [regex]::Replace($text, $Pattern, $Replacement)
    if ($updated -eq $text) {
        if ($Soft) { Write-Warning "pattern absent (soft, skipped): $Path"; return }
        Write-Error "pattern not found in ${Path}: $Pattern"
        exit 1
    }
    [System.IO.File]::WriteAllText($Path, $updated, [System.Text.UTF8Encoding]::new($hasBom))
    Write-Host "  synced: $Path"
}

Write-Host "bump $($chosen[0]): $old -> $new"

Update-File -Path $goPath `
    -Pattern 'const Version = "\d+\.\d+\.\d+"' `
    -Replacement ('const Version = "' + $new + '"')

Update-File -Path (Join-Path $root 'wails.json') `
    -Pattern '("productVersion":\s*")\d+\.\d+\.\d+(")' `
    -Replacement ('${1}' + $new + '${2}')

Update-File -Path (Join-Path $root 'frontend/package.json') `
    -Pattern '("version":\s*")\d+\.\d+\.\d+(")' `
    -Replacement ('${1}' + $new + '${2}')

Update-File -Path (Join-Path $root 'docs/DESIGN.md') `
    -Pattern '(版本：)\d+\.\d+\.\d+' `
    -Replacement ('${1}' + $new)

Update-File -Path (Join-Path $root 'build/windows/installer/wails_tools.nsh') `
    -Pattern '(!define INFO_PRODUCTVERSION ")\d+\.\d+\.\d+(")' `
    -Replacement ('${1}' + $new + '${2}') `
    -Soft

# verify source of truth
$verify = [System.IO.File]::ReadAllText($goPath)
if ($verify -notmatch ('const Version = "' + [regex]::Escape($new) + '"')) {
    Write-Error "verification failed: version.go does not contain $new"
    exit 1
}

Write-Host "done: version = $new"
Write-Host "next: append a row to the history table in docs/VERSIONING.md, then build (wails build [-nsis])."
