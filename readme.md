# SetSystemTime (Go 版本)

Windows 系统时间修改工具，使用纯 Go + Win32 API 实现，无外部依赖。

## 特性

- ✅ 纯 Go 实现，无外部依赖
- ✅ 原生 Win32 控件（日期选择器、按钮）
- ✅ 通过 Windows API 直接设置系统时间
- ✅ 支持 NTP 时间同步恢复
- ✅ 编译后仅 ~2.5MB
- ✅ 需要管理员权限运行

## 构建

```bash
# 直接编译
go build -o SetSystemTime.exe .

# 或使用构建脚本
build.bat
```

## 使用

1. **以管理员身份运行** `SetSystemTime.exe`
2. 在日期选择器中选择目标日期
3. 点击 **「设置日期」** 按钮修改系统时间
4. 点击 **「恢复时间」** 按钮同步 NTP 网络时间

## 嵌入管理员权限请求（可选）

默认需要右键「以管理员身份运行」。如需双击自动请求管理员权限：

```bash
# 安装 go-winres 工具
go install github.com/tc-hib/go-winres@latest

# 生成资源文件
go-winres make --manifest app.manifest

# 重新编译
go build -ldflags="-s -w -H windowsgui" -o SetSystemTime.exe .
```

## 技术实现

| 组件 | 实现方式 |
|------|---------|
| GUI | Win32 API (user32.dll) |
| 日期选择器 | SysDateTimePick32 控件 |
| 设置时间 | SetLocalTime API (kernel32.dll) |
| 恢复时间 | w32tm /resync 命令 |
| 字体 | Microsoft YaHei |

## 与 Electron 版本对比

| 维度 | Electron 版 | Go 版 |
|------|------------|-------|
| 包体积 | ~80MB | ~2.5MB |
| 内存占用 | ~100MB | ~5MB |
| 启动速度 | ~2s | 即时 |
| 依赖 | Node.js + Chromium | 无 |
| 外观 | 自定义 HTML/CSS | Windows 原生控件 |
