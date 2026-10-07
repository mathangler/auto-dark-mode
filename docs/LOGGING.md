# 诊断日志手册（app.log）— 给未来的排查者

> 用途：如果"睡眠唤醒后任务栏卡深 / 翻深闪烁 / 主题不切换"类问题**再次出现**，
> 用户告诉你现象后，先读本文，再看 `%APPDATA%\AutoDarkMode\app.log`，
> 应能在 10 分钟内定位到具体是哪一环坏了。

## 0. 一段话背景（必读）

2026-09 修复过一轮该类 bug，共发现**两个根因**（详见 README「Known Issues」与
`docs/DESIGN.md` 3.2 节）：

1. **根因 #1（历史，已修）**：唤醒探测用有偏的 `GetTickCount64` 对比墙钟 → 探测从未触发
   → 唤醒后立刻写主题，Shell 未就绪错过唯一一次真实跳变广播 → 任务栏永久卡深。
2. **根因 #2（2026-09 实锤，已修）**：Go ticker 在睡眠期间错过的 deadline 会在唤醒瞬间
   **背靠背连发**——第一个 tick 正确走"检测到唤醒→延迟 8 秒"，但第二个 tick 墙钟差仅
   0.5 秒、被合法判为"没睡过" → **抢跑写入**（唤醒 +0.5s）→ 8 秒延迟被架空、跳变投递撞上
   还没起来的任务栏（`poke: delivered to 0`）、45 秒安全网因"无跳变"**没武装**。
   修复 = 唤醒应用待决期间所有 `tick` 让行（`tick: deferred`）。
3. **兜底现状**：45 秒安全网现在带**像素门禁**（采样任务栏真实颜色，已正确则跳过翻转=
   零闪动），翻转后有 +2s/+5s 投递阶梯，愈合窗口结束还有"最终定向戳"封恢复悬崖。
4. **视觉看门狗（1.3.0）**：真实切换后 +45s/+2m 无注册表补戳、+1m/+3m/+8m/+15m 像素采样
   ——**仅高置信失配才翻转**（不确定只补戳，绝不误闪）；封住"3 分钟愈合窗口关闭后外壳
   复发失步、无人能救"的残余洞（2026-09-24 实测复发一次即此类），复发最迟 15 分钟自愈。

本文所描述的日志行为自 2026-09-23 起由**当前安装版**（Program Files 内）持续产生。
（排查期间用过的独立取证版 `autodark-diag-orig.exe` 已按要求删除；现行任何
`wails build` 产物都自带本日志，无需专门构建。）

## 1. 日志文件

| 文件 | 内容 | 格式 |
|---|---|---|
| `%APPDATA%\AutoDarkMode\app.log` | **诊断日志**（本文主角） | `[YYYY-MM-DD HH:MM:SS.mmm] 内容`，毫秒精度 |
| `%APPDATA%\AutoDarkMode\app.log.1` / `.2` | 轮转的历史代 | 达 1MB 轮转，共保留 3 代 |
| `%APPDATA%\AutoDarkMode\startup.log` | 仅启动/退出/单实例拦截（4 个固定事件） | 秒精度，另一套体系 |

代码入口：`diaglog.go` 的 `diagLog()`；快照函数 `themeSnapshot()` 输出
`apps=<dark|light> system=<dark|light>`（**写日志当时**的注册表真实值，0=dark 1=light）。

## 2. 健康唤醒的基准时间线（背下这个，异常才显眼）

以下摘自 2026-09-23 09:12 的真实隔夜唤醒（深色睡 4.5 小时 → 浅色唤醒，solar 模式）：

```
09:12:13.443  tick: RESUME detected (wallGap=4h30m28s runtimeGap=19s) -> arm wake apply, NO registry write now [apps=dark system=dark]
09:12:14.005  wake apply: armed, fires in 8s [apps=dark ...]
09:12:14.358  power: resume event code=0x7 -> scheduleWakeHeal      ← 电源事件通道（可能有 0x7/0x8/0x9/0x12 多条）
09:12:15.846  tick: deferred, wake apply pending [apps=dark ...]     ← 关键！第二个 tick 必须是 deferred
09:12:15.862  wake apply: already armed (coalesced)                  ← 多路信号合并
09:12:22.134  wake apply: 8s elapsed -> running applyWakeTheme       ← T+8s
09:12:22.135  wake apply: target=light before=true [...]
09:12:22.505  wake apply: applied dark=false REAL transition -> schedule stuck heal (+45s) [apps=light system=light]
09:12:22.508  stuck heal: armed, fires in 45s (target dark=false)
09:13:07.579  stuck heal: pixel gate PASSED (median=238 ... -> already correct, SKIP flip (poke only)) -> reinforce without flip   ← T+53s 零闪动
09:15:44.782  tick: healing window ENDED -> final targeted poke [apps=light system=light]   ← T+3min 悬崖封口
```

期间每 30 秒应有 `tick: healing window, registry matches -> re-broadcast` + `poke: delivered to 1 taskbar window(s)`；
每次真实写入都有 `registry: ALL keys dark=X -> apps=... system=...`；广播都有 `broadcast: WM_SETTINGCHANGE ...`。

## 3. 事件目录（按前缀 grep 即可）

### 调度与唤醒链（app.go / tray.go）
| 前缀行 | 含义 |
|---|---|
| `startup: version=... mode=... enabled=...` | 进程启动与**版本归属** + 配置快照（对照当前应然行为；版本规则见 [VERSIONING.md](VERSIONING.md)） |
| `config saved:` / `user: enabled ->` / `user: manualTheme ->` | 用户改了配置/开关/强制主题（含改后注册表快照） |
| `tick: RESUME detected (wallGap=... runtimeGap=...)` | 无偏计数器探测到睡眠（>30s 缺失运行时间即判定） |
| `tick: deferred, wake apply pending` | **让行门禁生效**（根因 #2 的修复点；唤醒后 10 秒内必须出现） |
| `tick: mismatch -> applyTheme dark=X` | 常规路径：注册表≠目标 → 写入（若它紧跟在 RESUME 之后 1 秒内 = **根因 #2 复发**） |
| `tick: healing window, registry matches -> re-broadcast` | 唤醒/切换后 3 分钟内的周期补发（每 30s 一条） |
| `tick: healing window ENDED -> final targeted poke` | 愈合窗口关闭边沿的"最终一戳"（恢复悬崖封口） |
| `tick: skip, flip heal in flight` | 翻转进行中，调度器让位（正常） |
| `tick: desiredTheme error, nothing applied` | **计划算不出目标**（solar 定位失败等）→ 不切换主题 |
| `power: resume event code=0x..` | 托盘窗口收到 PBT_APMRESUME*（0x6/0x7/0x8/0x9/0x12） |
| `power: away-mode LEAVE/ENTER` | 现代待机唤醒方向/入睡方向（ENTER 不做任何事=设计如此） |
| `power: wake burst coalesced` | 5 秒内重复唤醒信号被合并（正常） |
| `location: loaded disk cache / background resolve start (reason) / done / failed / coordinate jump REJECTED|CONFIRMED ...` | 定位：启动秒读 `location_cache.json`；**解析全在后台**（绝不同步——曾致黑窗卡死 ~30s）。done/failed 行**带坐标**；成功缓存 **6 小时**才刷新（10 分钟版曾 24/7 每 10.5 分钟探一次、一天 63 次）、失败 10 分钟重试且保留旧坐标；**单次跳变 >2° 拒收**、10 分钟后二次确认（连续一致才采纳）——防实测的 1000km 级坏 fix |

### 8 秒延迟应用（`wake apply:`）
| 行 | 含义 |
|---|---|
| `armed, fires in 8s [...]` | 武装（首个唤醒信号，带注册表快照） |
| `already armed (coalesced)` | 合并重复信号（正常） |
| `8s elapsed -> running applyWakeTheme` | **T+8s 到点执行**。防睡眠穿透：8 秒按实际运行时间计——若机器中途微睡，此行会晚于 T+8 墙钟出现 |
| `target=X before=Y [...]` | 计算出的目标与应用前状态 |
| `applied dark=X REAL transition -> schedule stuck heal (+45s)` | **真实跳变** → 武装 45 秒安全网 |
| `applied dark=X no transition` | 注册表本已一致 → **不武装安全网**。若前面刚发生过 `tick: mismatch`（抢跑），这就是根因 #2 的连带证据 |
| `skip (target=..., flipActive=...)` / `auto disabled -> skip` | 合法跳过 |

### 45 秒安全网（`stuck heal:`）
| 行 | 含义 |
|---|---|
| `armed, fires in 45s (target dark=X)` | 武装（仅真实跳变后） |
| `CANCELLED (another flip in flight / registry moved / user forced / auto switching disabled / schedule moved on)` | 五种合法取消，括号内是具体原因 |
| `pixel gate PASSED (median=... -> already correct, SKIP flip)` | **门禁判定任务栏已正确 → 跳过翻转（零闪动）**，只补投递 |
| `pixel gate (median=... -> flip)` | 读数模糊(中位数 111~169)或确实卡住 → 执行翻转 |
| `gates passed -> flipHealTheme` | 执行翻转（最终防线；其后必有 `flip:` 系列行） |
| `watchdog: armed (cause) / [label]: reinforce poke / CONFIDENT VISUAL MISMATCH -> heal / not confidently wrong -> reinforce / abort / session complete` | **视觉看门狗**（1.3.0，`watchdog.go`）：真实切换后 +45s/+2m 补戳、+1m/+3m/+8m/+15m 采样。**仅高置信失配才翻转**（正确或读数模糊只补戳——绝不误闪）；用户接管/自动关闭/定位失败即中止会话 |

### 翻转与投递（theme.go / taskbarpix.go）
| 行 | 含义 |
|---|---|
| `flip: START dark=X (system-side round trip, dwell 3s)` | 翻转开始（只翻系统侧键，应用不闪） |
| `flip: step1 delivered, dwelling 3s with system=X` → `flip: target written, final delivery` → `flip: DONE` | 三步完成；间隔应为 ~3s |
| `flip: ladder re-delivery +2s/5s [...]` | 翻回后的投递阶梯（保"翻回"必达） |
| `registry: ALL keys dark=X -> apps=.. system=..` | 全键写入 + **写后快照**（快照应等于 dark=X） |
| `registry: SYSTEM-ONLY dark=X -> ...` | 仅系统侧写入（翻转第一步/手动愈合） |
| `registry: ... write FAILED` | **写注册表失败**（权限/占用——写入侧故障的直接证据） |
| `broadcast: WM_SETTINGCHANGE ImmersiveColorSet -> HWND_BROADCAST` | 一次全局广播 |
| `poke: delivered to N taskbar window(s)` | 定向戳任务栏（**N=0 表示任务栏窗口此刻不存在**——唤醒早期属正常，长期为 0 = explorer 异常） |
| `tail: re-broadcast tail started (gaps 2/5/10/20/40s)` | 愈合长尾启动（每次真实切换/唤醒一次） |
| `wait: only Xs of the requested window ran -> re-waiting ...` | **防睡眠穿透生效的信号**：等待期间机器睡过/微睡，剩余时间重等（常见于现代待机微睡，属正常） |
| `heal-apply: registry already matches -> flip heal` | 托盘"强制深/浅"在注册表已匹配时走翻转式愈合 |
| `pixel sampler: unavailable, missing procedures: ...` | 像素采样 API 缺失 → 门禁降级为盲翻（出现在每次 stuck heal 前，只打一次） |

## 4. 病态签名速查

| 疑似问题 | 日志特征（按时间序） |
|---|---|
| **根因 #2 复发**（让行门禁失效） | `tick: RESUME detected` 之后 **1 秒内**出现 `tick: mismatch` 且**没有** `tick: deferred`；随后 `poke: delivered to 0`、`wake apply: ... no transition`、**全程无** `stuck heal: armed` |
| **根因 #1 复发**（唤醒探测失效） | 唤醒后**完全没有** `tick: RESUME detected` / `power: resume` / `wake apply: armed` 任何一行，直接出现 `tick: mismatch` |
| **任务栏卡深（写入侧）** | `registry: ... write FAILED`，或注册表快照本身与目标不符且无 `registry: ALL keys` 写入行 |
| **任务栏卡深（视觉失步）** | 注册表快照已是目标色，但用户看是旧色：查 `poke: delivered to 0`（没送达）、`stuck heal: CANCELLED (...)`（安全网被取消，看具体原因）、`pixel gate` 行的 median 是否被误判 |
| **闪深** | `stuck heal: pixel gate (median=...)`（非 PASSED）→ `flip: START`。若门禁 PASSED 仍闪 = 门禁误判（把 median 值与截图对照阈值 110/170） |
| **完全不切主题** | 反复 `tick: desiredTheme error` + `location: ... err=非空`（定位失败）；或 `wake apply pending` 持续超过 15 秒仍无 `elapsed`（标志位卡死——不该发生，发生了就是新 bug） |
| **现代待机微睡误判** | 大量 `wait: ... re-waiting` 但 `runtimeGap < 30s` 未触发 RESUME——正常，阈值设计即防此 |

## 5. 排查 Playbook（拿到现象后按序执行）

1. **让用户跑只读命令拿现场注册表**（点托盘修复**之前**！）：
   ```powershell
   Get-ItemProperty 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Themes\Personalize' | Select-Object AppsUseLightTheme,SystemUseLightTheme,SystemUsesLightTheme
   ```
   （0=dark 1=light。apps 跟应用、system 跟任务栏。）
2. 打开 `app.log`，**找到用户报告的唤醒时刻**（搜 `RESUME detected` 或报告时间），
   对照 §2 基准时间线逐行核对：armed → deferred → T+8 elapsed → REAL transition →
   stuck heal armed → pixel gate → healing window ENDED。
3. 哪一行缺失/变味 → 用 §4 签名表定位故障环 → 到 §3 查该行的确切含义。
4. 还不够 → 全文 grep `FAILED|CANCELLED|error|unavailable` 收集异常行。
5. 复现取证技巧：临时把配置切成固定时间模式、把"深→浅"边界设在当前时间 +6 分钟，
   合盖睡 6 分钟再开（跨边界睡眠），5 分钟内可稳定复现原场景（测完改回 solar）。

## 6. 关键常量与代码位置

| 常量 | 值 | 位置 |
|---|---|---|
| 调度周期 | 30s | `app.go` runLoop |
| 唤醒判定阈值（墙钟−运行时） | >30s | `app.go` `resumeDetected` |
| 唤醒延迟应用 | 8s（按实际运行时间） | `app.go` `wakeApplyDelay` |
| 愈合重播窗口 | 3min | `app.go` `wakeHealWindow` |
| 安全网延迟 | 45s | `app.go` `stuckHealDelay` |
| 翻转暗态驻留 | 3s（**勿改小**：两次反向写入过近会丢第二跳→复现卡死） | `theme.go` `flipHealPause` |
| 投递阶梯 | +2s / +5s | `theme.go` `runFlip` |
| 广播长尾 | 2/5/10/20/40s（防睡眠穿透） | `theme.go` `healShellAfterResume` |
| 像素采样 | 9×3 网格、亮度中位数；深色判定 <128；高置信 ≤110 / ≥170 | `taskbarpix.go` |
| 看门狗采样点 | +1m/+3m/+8m/+15m（补戳 +45s/+2m）；仅自信失配才翻转 | `watchdog.go` `visualWatchPlan` |
| 定位失败重试 | 每 10min（后台） | `app.go` `locationRetry` |
| 定位成功刷新 | 每 6h（后台） | `app.go` `locationRefresh` |
| 坐标跳变拒收/确认 | >2° 拒收 / ≤0.5° 二次确认 | `app.go` `locationJumpLimit/Confirm` |
| 日志轮转 | 1MB × 3 代 | `diaglog.go` |

事件产生点：`app.go`（tick/wake apply/stuck heal/watchdog 武装/配置/定位）、`theme.go`（registry/broadcast/tail/flip/poke/wait）、
`watchdog.go`（watchdog:*）、`tray.go`（power:*）、`taskbarpix.go`（pixel sampler）、`diaglog.go`（写入与轮转）。

## 7. 排查期的临时物（已清理，仅存档说明）

按用户要求已删除：源码备份 `_src_backup_pre_fix\`、取证版 `build\bin\autodark-diag-orig.exe`、
仓库根目录的旧构建 `autodark.exe`、配置备份 `config.json.forensic-orig`、
第三方应用孤儿数据 `_removed_3rdparty_backup\`。**当前 Program Files 安装版即含全部修复与本日志。**
