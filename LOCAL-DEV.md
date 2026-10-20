# magpie 本地开发环境

fork + upstream 同步 + 本地构建，全部已验证可用。

## 目录

| 路径 | 作用 |
| --- | --- |
| `~/Projects/magpie/` | 你的 fork（`keeuu/magpie`），`main` 跟官方对齐，改动在 `my/*` 分支 |
| `~/Projects/magpie-local-plugins/` | 本地私有插件，**不用改 magpie 源码** |
| `~/Applications/magpie.app` | 你编译的构建（跑 `dev-build.sh` 产出） |
| `/Applications/magpie.app` | 官方构建，仍可用 |

## 三个关键结论（都已实测）

### 1. 官方自动更新不会覆盖你的构建 ✅

magpie 更新时会跑 `sameSigner`：要求新包和旧包的 `TeamIdentifier` 一致。

- 官方构建：`TeamIdentifier=LY7MVTUDZG`
- 你的构建：adhoc 签名，`TeamIdentifier=not set`（空）

两者不等 → 官方包被拒。实测输出：
```
the downloaded app is signed by "LY7MVTUDZG", not ""
```

所以把自编译版本装到 `~/Applications`，官方那套自动更新碰不到它。

### 2. 大部分功能不用改源码，写本地插件就行 ✅

实测：装了一个本地中间件，它把请求里的 "MAGIC" 改写成指令，模型真的返回了 `LOCAL_PLUGIN_RAN`。全程没动 magpie 一行源码。

```sh
magpie plugin add /Users/zhangzeyuan/Projects/magpie-local-plugins/demo-header
```

插件发现规则（`internal/plugin/store.go:291`）：支持 `file://`、`.`、`/`、`\` 开头的路径，以及绝对路径。

**可用的扩展点**（都在 `docs/subsystems/` 有设计文档）：

| 想做什么 | 扩展点 | 语言 |
| --- | --- | --- |
| 改请求/回复、改路由策略、算用量 | gateway middleware（`onRequest`/`onEvent`/`onResponse`/`strategies`/`onUsage`） | JS，跑在 magpie 进程内（moejs） |
| 接新的模型供应商/订阅 | provider plugin（`auth` 钩子） | JS on Bun |
| 给 agent 提供工具 | plugin tools（MCP server） | JS |
| 加一个新设置页 | 内置 plugin + `Client` 前端资源 | Go + JS |
| 启动/停止时做点事 | `pluginrt.Manifest.Apply` + `OnDispose` | Go（内置 plugin） |

### 3. 上游更新极快，必须用分支策略

实测提交量：10-13 到 10-15 每天 100+，10-16 65，10-17 75，10-19 84。今天已到 v0.1.1296。

**别在 `main` 上改任何东西**，`main` 永远只做官方镜像。

## 日常流程

```sh
cd ~/Projects/magpie

# 1. 开工：从最新 main 切分支
git checkout main && git pull && git checkout -b my/某个功能

# 2. 写代码 / 写插件，本地验证
./dev-build.sh --cli     # 快速验证编译（约 2 分钟）
./dev-build.sh           # 完整构建 + 打包 + 签名 + 装到 ~/Applications

# 3. 提交
git add -A && git commit -m "feat: ..."

# 4. 官方更新了：同步 + 把你的分支 rebase 上去
./sync-upstream.sh          # 只同步当前分支
./sync-upstream.sh --all    # 同步所有 my/* 分支
./sync-upstream.sh --check  # 只看落后多少，不动分支
```

## 已踩过的坑

1. **端口冲突**：两个 magpie 实例会抢 `127.0.0.1:3425`。`dev-build.sh` 会先 `pkill` 旧实例。
2. **登录自启指向旧路径**：`~/Library/LaunchAgents/com.yetone.magpie.plist` 里写死了 `/Applications/magpie.app/...`。`dev-build.sh` 会自动改成新路径。
3. **GUI 编译要 cgo**：必须本机编译（链接 macOS webview），不能交叉编译。CLI 版（`-tags nogui`）可以随便交叉编译。
4. **首次编译慢**：Go 依赖下载约 2 分钟，之后有缓存。
5. **adhoc 签名会被 Gatekeeper 拦**：用 `open` 启动没问题，双击可能提示「无法验证开发者」。要永久解决得用 Apple Developer 账号签名。

## 两个选择：改源码 vs 写插件

**优先写插件。** 你的改动跟 magpie 源码完全隔离，上游怎么变都不冲突，升级零成本。

只有这些情况才需要改源码：
- 要动 GUI 主界面（`internal/gui/assets/app.js`，23895 行，改起来最容易冲突）
- 要加内置 plugin（Go，`plugins/` 下新建包 + 注册到 `plugins/all/all.go`）
- 要改网关核心行为（`internal/gateway/`）

改源码时，扩展点都是现成的，参考 `plugins/otel/otel.go`（最简内置插件，50 行）和 `plugins/usage/usage.go`（带前端页面的插件）。

## 环境

- Go 1.27.3（`brew install go`，已装）
- Node v26.8.2（GUI 测试用）
- Xcode Command Line Tools（cgo）
- gh CLI 已登录 `keeuu`
