# Headroom Kompress ML GPU 加速支持计划

> 创建日期：2026-07-03
> 状态：待实施

## 背景

Headroom 的 Kompress 压缩基于 ModernBERT（HuggingFace: `chopratejas/kompress-v2-base`），
通过 ONNX Runtime 运行。默认使用 CPU 推理，在核显/低配机器上延迟过高（9-20 秒），不适合实时聊天。

对于配备**独立显卡**（NVIDIA / AMD dGPU）的用户，GPU 推理可显著降低延迟。

## 当前状态

- `disable_kompress = true` 为所有预设的默认值
- Kompress ML 模型通过 HuggingFace 下载，需 VPN
- ONNX Runtime 已安装，`DmlExecutionProvider` 可用（AMD GPU）
- NVIDIA GPU 需 `CUDAExecutionProvider`（额外安装 `onnxruntime-gpu`）

## 目标

在 Settings UI 中提供 GPU 加速选项，用户可选择：

| 选项 | 效果 |
|------|------|
| 自动检测 | 检测可用 GPU（CUDA / DirectML），优先 GPU，fallback CPU |
| CPU only | 使用 CPU 推理 |
| DirectML（AMD） | 强制 AMD GPU |
| CUDA（NVIDIA） | 强制 NVIDIA GPU（需 `onnxruntime-gpu`） |

## 实施要点

### 后端

- `HeadroomConfig` 新增 `GpuBackend string` 字段：`auto` | `cpu` | `dml` | `cuda`
- `headroomSidecar.start()` 注入 `HEADROOM_GPU_BACKEND` 环境变量
- Windows launcher 中读取环境变量，patch `KompressCompressor._get_inference_sessions` 设置正确的 ONNX providers

### 前端

- Settings → Headroom 高级选项中新增 "GPU 加速" 下拉框
- 自动检测 GPU 可用性并显示提示

### 前置条件

- `disable_kompress = false` 且 HuggingFace 模型已下载
- `pip install onnxruntime-gpu`（NVIDIA 用户）

## 风险

- GPU 推理在 dGPU 上可能造成显存压力
- DirectML 在集成显卡上收益有限
- 模型下载需访问 HuggingFace（国内需代理）

## 关联文件

- `desktop/headroom_sidecar.go` — `writeWindowsLauncher()`, `start()`
- `internal/config/config.go` — `HeadroomConfig`
- `desktop/frontend/src/components/SettingsPanel.tsx` — `HeadroomSettingsSection`
