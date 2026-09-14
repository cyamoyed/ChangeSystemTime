@echo off
chcp 65001 >nul
echo === SetSystemTime Go 构建脚本 ===

cd /d "%~dp0"

echo [1/4] 检查 go-winres...
where go-winres >nul 2>&1
if errorlevel 1 (
    echo go-winres 未安装，正在安装...
    go install github.com/tc-hib/go-winres@latest
    if errorlevel 1 (
        echo 安装 go-winres 失败，请手动运行:
        echo   go install github.com/tc-hib/go-winres@latest
        pause
        exit /b 1
    )
    echo go-winres 安装成功
) else (
    echo go-winres 已就绪
)

echo [2/4] 生成资源文件...
go-winres make --in winres\winres.json --out rsrc --arch amd64
if errorlevel 1 (
    echo 生成资源文件失败！
    pause
    exit /b 1
)

echo [3/4] 编译程序...
set GOOS=windows
set GOARCH=amd64
set CGO_ENABLED=0
go build -ldflags="-s -w -H windowsgui" -o SetSystemTime.exe .
if errorlevel 1 (
    echo 编译失败！
    pause
    exit /b 1
)

echo [4/4] 清理临时文件...
if exist rsrc_windows_amd64.syso del rsrc_windows_amd64.syso
if exist rsrc_windows_386.syso del rsrc_windows_386.syso
if exist rsrc_windows_arm64.syso del rsrc_windows_arm64.syso

for %%A in (SetSystemTime.exe) do echo 文件大小: %%~zA bytes

echo.
echo === 构建成功！===
echo 输出文件: SetSystemTime.exe
echo.
echo 特性:
echo   - 双击自动请求管理员权限 (UAC)
echo   - 无 CMD 窗口弹出
echo   - 单文件 EXE，无外部依赖
echo.
pause
