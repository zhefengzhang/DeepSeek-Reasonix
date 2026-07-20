# Desktop 构建与打包指南

> 适用于 Windows amd64 平台。其他平台请参考 CI 配置 `.github/workflows/release-desktop.yml`。

## 环境准备

```bash
# Go 工具链（版本由 desktop/go.mod 指定）
go version

# Node.js ≥ 22 + pnpm 10（前端构建）
node --version
pnpm --version

# Wails CLI v2.12.0
go install github.com/wailsapp/wails/v2/cmd/wails@v2.12.0

# NSIS（可选，生成 Windows 安装包时需要）
# 下载：https://nsis.sourceforge.io/Download

# UPX（可选，压缩二进制文件）
# 下载：https://github.com/upx/upx/releases
# 解压后把 upx.exe 放到 PATH 中
```

## 快速构建（开发测试）

```bash
cd desktop

wails build \
  -platform windows/amd64 \
  -ldflags "-s -w -X main.version=0.0.0-dev -X main.channel=dev" \
  -trimpath
```

产物：`build/bin/reasonix-desktop.exe`

## 优化构建（Release 级别）

```bash
cd desktop

wails build -clean -platform windows/amd64 -ldflags "-s -w -X main.version=1.16.0-dev -X main.channel=dev" -trimpath -upx
```

产物：`build/bin/reasonix-desktop.exe`（UPX 压缩后约 13MB）

> 注意：`-clean` 会删除 `build/bin/` 目录，如果正在运行桌面程序，该文件会被锁定导致构建失败。
> 此时去掉 `-clean` 或使用 `-o reasonix-desktop-opt.exe` 输出到不同文件名。

## 完整打包（含安装包）

```bash
cd desktop

wails build \
  -clean \
  -platform windows/amd64 \
  -ldflags "-s -w -X main.version=1.0.0 -X main.channel=stable" \
  -trimpath \
  -upx \
  -nsis \
  -webview2 embed
```

产物：
- `build/bin/reasonix-desktop.exe` — 便携版
- `build/bin/*installer*.exe` — NSIS 安装包（含 WebView2 嵌入）

## 参数说明

| 参数 | 作用 | 必选 |
|------|------|------|
| `-platform windows/amd64` | 目标平台 | 是 |
| `-ldflags "-s -w"` | 剥离 DWARF 调试符号和符号表，减小体积 | 推荐 |
| `-ldflags "-X main.version=..."` | 注入版本号，影响 updater 行为 | 发布时必选 |
| `-ldflags "-X main.channel=..."` | 注入更新通道（stable / canary / dev） | 发布时必选 |
| `-trimpath` | 移除编译路径信息，实现可复现构建 | 推荐 |
| `-upx` | 使用 UPX 压缩二进制，体积减少约 70% | 推荐 |
| `-nsis` | 生成 NSIS Windows 安装包 | 发布时推荐 |
| `-webview2 embed` | 将 WebView2 引导程序嵌入安装包 | 发布时推荐 |
| `-clean` | 构建前清空 `build/bin/` | 可选 |

## 版本号规范

版本号格式遵循 SemVer：`v<major>.<minor>.<patch>`，例如 `1.2.3`。

```
# stable 发布
wails build -ldflags "-s -w -X main.version=1.2.3 -X main.channel=stable"

# canary 预发布
wails build -ldflags "-s -w -X main.version=1.2.3-canary.20260702.1 -X main.channel=canary"
```

## 常见问题

### 构建时提示 `Access is denied`

正在运行的桌面程序锁定了 `build/bin/reasonix-desktop.exe`。
解决方案：
1. 去掉 `-clean` 参数
2. 使用 `-o other-name.exe` 输出到不同文件名
3. 关闭桌面程序后再构建

### UPX 压缩后程序无法启动

UPX 5.0+ 对 Go/Wails 二进制兼容性良好。
如果遇到问题，尝试不加 `-upx` 构建，构建完成后手动压缩：

```bash
upx --best build/bin/reasonix-desktop.exe
```

### 前端资源未更新

强制重新构建前端：

```bash
cd desktop/frontend && pnpm build && cd .. && wails build ...
```

或者使用 Wails 的 `-s` 跳过前端构建（如果前端已是最新）：

```bash
wails build -s ...
```

## 对比：CI 使用的构建参数

CI 配置在 `.github/workflows/release-desktop.yml`，实际构建命令在 `scripts/desktop-build.sh`：

```bash
wails build \
  -clean \
  -platform windows/amd64 \
  -ldflags "-X main.version=$VERSION -X main.channel=$CHANNEL" \
  -nsis \
  -webview2 embed
```

> CI 不启用 `-upx`，但产物通过 `.zip` 打包发布。
> CI 不启用 `-s -w`，但通过 Go linker 默认行为已达到类似效果。
> 本指南推荐的命令是 CI 命令的超集，增加了 `-s -w -trimpath -upx` 优化。
