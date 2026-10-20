# KeyNest

[English](README.md) | 简体中文

兼容 KeePass 的免费跨平台密码管理器。

## 软件预览

![KeyNest 软件预览](docs/images/keynest-preview.png)

## 功能介绍

- 使用兼容 KeePass 的 KDBX 4.1 格式，本地保存保险库数据。
- 支持主密码、密钥文件或组合认证，并可修改主密码。
- 支持条目的新增、编辑、复制、收藏、搜索、历史版本及密码生成。
- 支持多级分组，通过拖拽调整分组层级、顺序和条目归属。
- 支持空闲自动锁定和回收站恢复，误删记录可在 30 天内找回。

## 数据存储

- 密码库使用兼容 KeePass 的 KDBX 4.1 格式。
- KDBX 文件由用户选择位置并保存在本机，不上传到云端。
- 支持主密码、密钥文件，或二者组合解锁。
- 主密码不会持久化；最近使用列表只记录 KDBX 文件路径。
- 新建和编辑条目时会加密并原子写回当前 KDBX 文件。
- 内置基于系统加密随机源的随机、易读和 PIN 密码生成器。

## 开发

```bash
make dev
```

需要更详细的调试日志时使用 `make debug`；生成带调试工具的应用包使用 `make build-debug`。

仅预览前端界面：

```bash
cd frontend
npm run dev
```

## 验证与构建

```bash
make test
make release
```

macOS 应用构建到 `build/bin/KeyNest.app`。

可通过 `BUILD_FLAGS` 传递额外的 Wails 构建参数，例如：

```bash
make release BUILD_FLAGS="-platform windows/amd64"
```

## 版本更新

macOS 用户应下载 `KeyNest-macOS-arm64.dmg`，打开后将 `KeyNest.app` 拖入“应用程序”文件夹。当前 macOS 安装包适用于 Apple Silicon。

正式构建会在“设置 → 关于与更新”中检查 GitHub Releases。自动更新仅接受与当前平台匹配的原始可执行文件，并在替换前验证同一 Release 中的 SHA-256 文件。原始可执行文件是自动更新载荷，不用于手动安装。

发布版本使用 `vMAJOR.MINOR.PATCH` 标签。推送标签后，`.github/workflows/release.yml` 会创建 Release 并上传以下文件：

- `KeyNest-macOS-arm64.dmg` 与 `KeyNest-macOS-arm64.dmg.sha256`（macOS 用户安装包）
- `keynest-darwin-arm64` 与 `keynest-darwin-arm64.sha256`
- `keynest-windows-amd64.exe` 与 `keynest-windows-amd64.exe.sha256`

本地 `wails dev` 和普通 `wails build` 属于开发构建，可以检查远程版本，但不会覆盖自身。`make release` 和发布工作流会从当前提交最近的 `vMAJOR.MINOR.PATCH` Git 标签读取版本，去掉 `v` 后写入应用界面、Go 二进制以及 macOS/Windows 包元数据，并通过链接参数写入 `BuildMode=production`，从而启用自动安装。

## 开源许可

KeyNest 采用 [MIT License](LICENSE) 许可证。
