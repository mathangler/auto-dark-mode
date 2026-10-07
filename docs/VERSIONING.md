# 版本迭代规则（VERSIONING）

> 本文是版本号的**唯一规则来源**。执行工具：`build\bump-version.ps1`。
> 出现疑问时先读这里；每次升位须先向用户报告"建议版本 + 理由"并获确认。

## 1. 方案

**SemVer** `MAJOR.MINOR.PATCH`，显示格式 `vX.Y.Z`（页脚）/ `X.Y.Z`（exe 属性、NSIS、日志）。

## 2. 升位规则（构建导向）

| 位 | 触发 | 例 |
|---|---|---|
| **MAJOR** | 破坏性：配置格式不兼容、移除/重命名既有功能、UI 结构性重做 | — |
| **MINOR** | 新增功能或行为机制（向后兼容） | 像素门禁、设置页页脚版本号 |
| **PATCH** | 修复 / 内部加固 / 日志与可观测性 / 构建链调整；**每个新 exe 构建前的保底 +1** | 恢复悬崖封口 |
| **不升位** | 纯文档 / 注释 / 聊天产出（无新 exe 产物） | LOGGING.md、本文自身 |

**铁律：每次产出新 exe 之前必须执行脚本（至少 `-Patch`）。**

## 3. 协作流程（我判断 → 你确认 → 执行）

1. 我判断本次改动性质，报告：**"建议升到 vX.Y.Z，理由：…"**；
2. 用户确认（或改判）；
3. 运行 `build\bump-version.ps1 -Patch|-Minor|-Major`（同步全部位置并打印结果；exe 属性与 NSIS 版本随 `wails.json` 构建时自动流转，无需额外步骤）；
4. 在本文 §5 历史表**追加一行**（日期 / 版本 / 一行要点）；
5. 构建（`wails build` 或 `wails build -nsis`）并验证（页脚 / exe 属性 / NSIS / 日志 startup 行）。

## 4. 同步位置清单（脚本管辖）

| 文件 | 位置 | 供给谁 |
|---|---|---|
| `version.go` | `const Version`（**真源**） | Go 后端、页脚（`GetVersion` 绑定）、`app.log` startup 行 |
| `wails.json` | `info.productVersion` | exe 文件属性（经 `build/windows/info.json` 模板）+ **NSIS `DisplayVersion`**（`wails build -nsis` 注入） |
| `frontend/package.json` | `version` | 前端包一致性（不对外显示） |
| `docs/DESIGN.md` | 首部"版本：" | 文档标称 |
| `build/windows/installer/wails_tools.nsh` | `INFO_PRODUCTVERSION` 兜底 | NSIS 手工构建兜底（**软目标**：该文件由 wails 生成，缺失模式仅告警） |

> exe 文件属性的链路是 `wails.json info.productVersion` → 构建时渲染 `info.json` 模板
> → winres 编入 syso，**bump 只需管上表 5 处**（info.json 内是模板，无需同步）。
> 注意 `info.json` 的 `info` 键必须是 4 位短 LCID `"0409"`（坑见 DESIGN §7）。

**版本呈现面**：设置窗口页脚 `vX.Y.Z`（静态文本）· 右键 exe→属性→详细信息 ·
控制面板/“应用和功能”（NSIS 卸载注册表 `DisplayVersion`）· `app.log` 的 `startup: version=...` 行。

## 5. 版本历史

| 日期 | 版本 | 要点 |
|---|---|---|
| 2026-09-15 | 1.0.0 | DESIGN.md 标称的初始版本（从未落到 exe 属性） |
| 2026-09-23 | 1.1.0 | 唤醒卡深双根因修复 + 常驻诊断日志 + 像素门禁 + 同类加固（已部署构建；当时 exe 属性仍显示 1.0.0，版本体系尚不存在） |
| 2026-09-23 | 1.2.0 | 首个版本化构建：设置页页脚版本号、`GetVersion` 绑定、`app.log` startup 行带版本、本规则文档与升位脚本 |
| 2026-09-23 | 1.2.1 | 修复启动黑窗/点击无响应：定位解析移出 Wails 主循环 + `location_cache.json` 磁盘缓存秒回 + 启动首 tick 异步 + 刷新失败保留旧坐标 |
| 2026-09-24 | 1.3.0 | 视觉看门狗（+45s/+2m 补戳、+1m/+3m/+8m/+15m 采样，**仅高置信失配才翻转**）封"愈合窗口后外壳复发"洞；定位：成功缓存 6h 节流（终结 10 分钟无限刷新）、坐标跳变 >2° 拒收需二次确认（防 1000km 级坏 fix）、解析日志带坐标；冷启动重复解析守卫（1.2.1 攒批） |
