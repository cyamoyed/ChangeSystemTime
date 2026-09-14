package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	gdi32    = syscall.NewLazyDLL("gdi32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	shell32  = syscall.NewLazyDLL("shell32.dll")
	advapi32 = syscall.NewLazyDLL("advapi32.dll")

	// Window 管理
	pRegisterClassExW      = user32.NewProc("RegisterClassExW")
	pCreateWindowExW       = user32.NewProc("CreateWindowExW")
	pShowWindow            = user32.NewProc("ShowWindow")
	pUpdateWindow          = user32.NewProc("UpdateWindow")
	pGetMessageW           = user32.NewProc("GetMessageW")
	pTranslateMessage      = user32.NewProc("TranslateMessage")
	pDispatchMessageW      = user32.NewProc("DispatchMessageW")
	pDefWindowProcW        = user32.NewProc("DefWindowProcW")
	pPostQuitMessage       = user32.NewProc("PostQuitMessage")
	pSendMessageW          = user32.NewProc("SendMessageW")
	pPostMessageW          = user32.NewProc("PostMessageW")
	pDestroyWindow         = user32.NewProc("DestroyWindow")
	pGetClientRect         = user32.NewProc("GetClientRect")
	pLoadCursorW           = user32.NewProc("LoadCursorW")
	pMessageBoxW           = user32.NewProc("MessageBoxW")
	pEnableWindow          = user32.NewProc("EnableWindow")
	pSetWindowTextW        = user32.NewProc("SetWindowTextW")
	pMoveWindow            = user32.NewProc("MoveWindow")
	pGetDpiForSystem       = user32.NewProc("GetDpiForSystem")
	pSystemParametersInfoW = user32.NewProc("SystemParametersInfoW")
	pGetDC                 = user32.NewProc("GetDC")
	pReleaseDC             = user32.NewProc("ReleaseDC")

	// GDI
	pCreateFontW   = gdi32.NewProc("CreateFontW")
	pDeleteObject  = gdi32.NewProc("DeleteObject")
	pGetDeviceCaps = gdi32.NewProc("GetDeviceCaps")

	// 系统时间
	pSetSystemTime    = kernel32.NewProc("SetSystemTime")
	pGetModuleHandleW = kernel32.NewProc("GetModuleHandleW")

	// 提权相关
	pShellExecuteW    = shell32.NewProc("ShellExecuteW")
	pOpenProcessToken = advapi32.NewProc("OpenProcessToken")
	pGetTokenInfo     = advapi32.NewProc("GetTokenInformation")
)

// Windows 常量
const (
	WS_OVERLAPPED    = 0x00000000
	WS_CAPTION       = 0x00C00000
	WS_SYSMENU       = 0x00080000
	WS_MINIMIZEBOX   = 0x00020000
	WS_MAXIMIZEBOX   = 0x00010000
	WS_THICKFRAME    = 0x00040000
	WS_VISIBLE       = 0x10000000
	WS_CHILD         = 0x40000000
	WS_VSCROLL       = 0x00200000
	WS_EX_CLIENTEDGE = 0x00000200

	ES_MULTILINE   = 0x0004
	ES_AUTOVSCROLL = 0x0040
	ES_READONLY    = 0x0800
	EM_SETSEL      = 0x00B1
	EM_REPLACESEL  = 0x00C2
	EM_SCROLLCARET = 0x00B7

	SW_SHOW = 5

	WM_DESTROY = 0x0002
	WM_CLOSE   = 0x0010
	WM_COMMAND = 0x0111
	WM_SETFONT = 0x0030
	WM_SIZE    = 0x0005
	WM_APP     = 0x8000

	BN_CLICKED = 0

	IDC_ARROW = 32512

	COLOR_BTNFACE = 15

	DTM_GETSYSTEMTIME = 0x1001
	GDT_VALID         = 0

	MB_OK              = 0x00000000
	MB_ICONERROR       = 0x00000010
	MB_ICONINFORMATION = 0x00000040

	ID_BTN_SET     = 1001
	ID_BTN_RESTORE = 1002
	ID_DATEPICKER  = 2001
	ID_LOG         = 1003
	ID_STATUS      = 1004
	ID_LABEL       = 1005

	WM_ASYNC_RESULT = WM_APP + 1

	asyncOK  = 0
	asyncErr = 1

	TOKEN_QUERY    = 0x0008
	TokenElevation = 20

	ERROR_CLASS_ALREADY_EXISTS = 1410

	SPI_GETWORKAREA = 0x0030
	LOGPIXELSX      = 88

	CMD_TIMEOUT = 15 * time.Second

	// 基准尺寸（96 DPI 下的逻辑像素，按 DPI 缩放）
	baseWinW = 480
	baseWinH = 420

	ntpClientRegKey = `HKLM\SYSTEM\CurrentControlSet\Services\W32Time\TimeProviders\NtpClient`
)

type SYSTEMTIME struct {
	Year, Month, DayOfWeek, Day, Hour, Minute, Second, Milliseconds uint16
}

type WNDCLASSEXW struct {
	Size       uint32
	Style      uint32
	WndProc    uintptr
	ClsExtra   int32
	WndExtra   int32
	Instance   uintptr
	Icon       uintptr
	Cursor     uintptr
	Background uintptr
	MenuName   *uint16
	ClassName  *uint16
	IconSm     uintptr
}

type MSG struct {
	HWnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      struct{ X, Y int32 }
}

type RECT struct {
	Left, Top, Right, Bottom int32
}

type TOKEN_ELEVATION struct {
	TokenIsElevated uint32
}

var (
	hDatePicker uintptr
	hBtnSet     uintptr
	hBtnRestore uintptr
	hLog        uintptr
	hStatus     uintptr
	hLabel      uintptr
	hFont       uintptr
	hWndMain    uintptr

	dpi = 96

	opMu         sync.Mutex
	opInProgress bool

	asyncMu         sync.Mutex
	asyncResultMsg  string
	asyncResultCode int

	w32Mu      sync.Mutex
	w32Stopped bool
)

// ---- 提权 ----

func isRunningAsAdmin() bool {
	var token syscall.Token
	proc, _ := syscall.GetCurrentProcess()
	ret, _, _ := pOpenProcessToken.Call(uintptr(proc), TOKEN_QUERY, uintptr(unsafe.Pointer(&token)))
	if ret == 0 {
		return false
	}
	defer token.Close()
	var elevation TOKEN_ELEVATION
	var retLen uint32
	ret, _, _ = pGetTokenInfo.Call(uintptr(token), TokenElevation,
		uintptr(unsafe.Pointer(&elevation)), unsafe.Sizeof(elevation), uintptr(unsafe.Pointer(&retLen)))
	return ret != 0 && elevation.TokenIsElevated != 0
}

func selfElevate() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	verb, _ := syscall.UTF16PtrFromString("runas")
	file, _ := syscall.UTF16PtrFromString(exe)
	params, _ := syscall.UTF16PtrFromString("-elevated")
	ret, _, callErr := pShellExecuteW.Call(0,
		uintptr(unsafe.Pointer(verb)),
		uintptr(unsafe.Pointer(file)),
		uintptr(unsafe.Pointer(params)), 0, SW_SHOW)
	if ret <= 32 {
		return fmt.Errorf("请求管理员权限失败 (ShellExecute=%d): %v", ret, callErr)
	}
	return nil
}

// ---- DPI / 布局 ----

func systemDPI() int {
	if err := pGetDpiForSystem.Find(); err == nil {
		ret, _, _ := pGetDpiForSystem.Call()
		if ret >= 96 {
			return int(ret)
		}
	}
	hdc, _, _ := pGetDC.Call(0)
	if hdc != 0 {
		defer pReleaseDC.Call(0, hdc)
		ret, _, _ := pGetDeviceCaps.Call(hdc, LOGPIXELSX)
		if ret >= 96 {
			return int(ret)
		}
	}
	return 96
}

func scale(v int) int {
	return (v*dpi + 48) / 96
}

func scaleI32(v int) int32 {
	return int32(scale(v))
}

func workArea() RECT {
	var wa RECT
	pSystemParametersInfoW.Call(SPI_GETWORKAREA, 0, uintptr(unsafe.Pointer(&wa)), 0)
	return wa
}

func centeredPos(w, h int) (int32, int32) {
	wa := workArea()
	ww := wa.Right - wa.Left
	wh := wa.Bottom - wa.Top
	x := wa.Left + (ww-int32(w))/2
	y := wa.Top + (wh-int32(h))/2
	if x < wa.Left {
		x = wa.Left
	}
	if y < wa.Top {
		y = wa.Top
	}
	return x, y
}

func layoutControls(cx, cy int32) {
	m := int32(scale(16))
	gap := int32(scale(10))
	labelH := int32(scale(22))
	pickH := int32(scale(30))
	btnH := int32(scale(36))
	statusH := int32(scale(22))
	innerW := cx - 2*m
	if innerW < 100 {
		innerW = 100
	}

	y := m
	pMoveWindow.Call(hLabel, uintptr(m), uintptr(y), uintptr(innerW), uintptr(labelH), 1)
	y += labelH + gap

	pMoveWindow.Call(hDatePicker, uintptr(m), uintptr(y), uintptr(innerW), uintptr(pickH), 1)
	y += pickH + gap

	btnW := int32(scale(140))
	btnGap := int32(scale(16))
	totalBtn := btnW*2 + btnGap
	btnX := (cx - totalBtn) / 2
	if btnX < m {
		btnX = m
	}
	pMoveWindow.Call(hBtnSet, uintptr(btnX), uintptr(y), uintptr(btnW), uintptr(btnH), 1)
	pMoveWindow.Call(hBtnRestore, uintptr(btnX+btnW+btnGap), uintptr(y), uintptr(btnW), uintptr(btnH), 1)
	y += btnH + gap

	statusY := cy - m - statusH
	logH := statusY - gap - y
	if logH < scaleI32(80) {
		logH = scaleI32(80)
	}
	pMoveWindow.Call(hLog, uintptr(m), uintptr(y), uintptr(innerW), uintptr(logH), 1)
	pMoveWindow.Call(hStatus, uintptr(m), uintptr(statusY), uintptr(innerW), uintptr(statusH), 1)
}

// ---- 工具 ----

func getModuleHandle() uintptr {
	ret, _, _ := pGetModuleHandleW.Call(0)
	return ret
}

func runCmdWithTimeout(name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), CMD_TIMEOUT)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	output, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return string(output), fmt.Errorf("命令超时 (%v): %s", CMD_TIMEOUT, name)
	}
	return string(output), err
}

func setNTPAutoEnabled(enabled bool) {
	val := "0"
	if enabled {
		val = "1"
	}
	runCmdWithTimeout("reg", "add", ntpClientRegKey, "/v", "Enabled", "/t", "REG_DWORD", "/d", val, "/f")
}

func markW32Stopped() {
	w32Mu.Lock()
	w32Stopped = true
	w32Mu.Unlock()
}

func markW32Started() {
	w32Mu.Lock()
	w32Stopped = false
	w32Mu.Unlock()
}

func restoreW32IfStopped() {
	w32Mu.Lock()
	need := w32Stopped
	w32Stopped = false
	w32Mu.Unlock()
	if !need {
		return
	}
	setNTPAutoEnabled(true)
	runCmdWithTimeout("net", "start", "w32time")
	waitW32Ready()
}

func waitW32Ready() {
	for i := 0; i < 10; i++ {
		out, err := runCmdWithTimeout("sc", "query", "w32time")
		if err == nil && containsRunning(out) {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func containsRunning(s string) bool {
	return strings.Contains(s, "RUNNING")
}

func setSystemTime(year int, month time.Month, day int) error {
	setNTPAutoEnabled(false)
	runCmdWithTimeout("net", "stop", "w32time")
	markW32Stopped()

	now := time.Now()
	local := time.Date(year, month, day, now.Hour(), now.Minute(), now.Second(), now.Nanosecond(), time.Local)
	utc := local.UTC()

	st := SYSTEMTIME{
		Year: uint16(utc.Year()), Month: uint16(utc.Month()), Day: uint16(utc.Day()),
		Hour: uint16(utc.Hour()), Minute: uint16(utc.Minute()),
		Second: uint16(utc.Second()), Milliseconds: uint16(utc.Nanosecond() / 1000000),
	}
	ret, _, err := pSetSystemTime.Call(uintptr(unsafe.Pointer(&st)))
	if ret == 0 {
		restoreW32IfStopped()
		return fmt.Errorf("SetSystemTime 失败: %v", err)
	}
	return nil
}

func syncNTPTime() error {
	setNTPAutoEnabled(true)
	runCmdWithTimeout("net", "start", "w32time")
	markW32Started()
	waitW32Ready()

	var output string
	var err error
	for i := 0; i < 3; i++ {
		output, err = runCmdWithTimeout("w32tm", "/resync", "/force")
		if err == nil {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("NTP 同步失败: %s\n%v", output, err)
}

func getDateFromPicker() (int, time.Month, int, error) {
	var st SYSTEMTIME
	ret, _, _ := pSendMessageW.Call(hDatePicker, DTM_GETSYSTEMTIME, 0, uintptr(unsafe.Pointer(&st)))
	if ret != GDT_VALID {
		return 0, 0, 0, fmt.Errorf("日期控件返回无效值")
	}
	if st.Year == 0 || st.Month < 1 || st.Month > 12 || st.Day < 1 {
		return 0, 0, 0, fmt.Errorf("无效日期")
	}
	return int(st.Year), time.Month(st.Month), int(st.Day), nil
}

func setStatus(text string) {
	if hStatus != 0 {
		pSetWindowTextW.Call(hStatus, uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(text))))
	}
}

func setButtonsEnabled(enabled bool) {
	var flag uintptr
	if enabled {
		flag = 1
	}
	pEnableWindow.Call(hBtnSet, flag)
	pEnableWindow.Call(hBtnRestore, flag)
}

func tryBeginOp() bool {
	opMu.Lock()
	defer opMu.Unlock()
	if opInProgress {
		return false
	}
	opInProgress = true
	return true
}

func endOp() {
	opMu.Lock()
	opInProgress = false
	opMu.Unlock()
}

func asyncPostResult(code int, msg string) {
	asyncMu.Lock()
	asyncResultMsg = msg
	asyncResultCode = code
	asyncMu.Unlock()
	pPostMessageW.Call(hWndMain, WM_ASYNC_RESULT, 0, 0)
}

func showMsg(hwnd uintptr, text, title string, flags uintptr) {
	pMessageBoxW.Call(hwnd,
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(text))),
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(title))),
		flags)
}

// appendLog 在窗口日志区追加一行（仅 UI 线程调用）
func appendLog(text string) {
	if hLog == 0 {
		return
	}
	line := fmt.Sprintf("[%s] %s\r\n", time.Now().Format("15:04:05"), text)
	allOnes := ^uintptr(0)
	pSendMessageW.Call(hLog, EM_SETSEL, allOnes, allOnes)
	pSendMessageW.Call(hLog, EM_REPLACESEL, 0, uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(line))))
	pSendMessageW.Call(hLog, EM_SCROLLCARET, 0, 0)
}

// wndProc 窗口过程 — 保持轻量，耗时工作放到 goroutine
func wndProc(hwnd, msg, wParam, lParam uintptr) uintptr {
	switch msg {
	case WM_COMMAND:
		if wParam>>16 == BN_CLICKED {
			switch wParam & 0xFFFF {
			case ID_BTN_SET:
				if !tryBeginOp() {
					return 0
				}
				year, month, day, err := getDateFromPicker()
				if err != nil {
					endOp()
					appendLog("失败：" + err.Error())
					setStatus("操作失败")
					return 0
				}
				setButtonsEnabled(false)
				setStatus("正在关闭自动同步并设置日期...")
				appendLog(fmt.Sprintf("开始设置日期为 %d-%02d-%02d ...", year, month, day))
				go func() {
					err := setSystemTime(year, month, day)
					if err != nil {
						asyncPostResult(asyncErr, err.Error())
					} else {
						asyncPostResult(asyncOK, fmt.Sprintf("已设置为 %d-%02d-%02d", year, month, day))
					}
				}()
			case ID_BTN_RESTORE:
				if !tryBeginOp() {
					return 0
				}
				setButtonsEnabled(false)
				setStatus("正在同步网络时间...")
				appendLog("开始恢复网络时间 ...")
				go func() {
					err := syncNTPTime()
					if err != nil {
						asyncPostResult(asyncErr, err.Error())
					} else {
						asyncPostResult(asyncOK, "系统时间已同步恢复")
					}
				}()
			}
		}
		return 0

	case WM_ASYNC_RESULT:
		asyncMu.Lock()
		code := asyncResultCode
		text := asyncResultMsg
		asyncMu.Unlock()
		endOp()
		setButtonsEnabled(true)
		if code == asyncOK {
			setStatus("就绪")
			appendLog("成功：" + text)
		} else {
			setStatus("操作失败")
			appendLog("失败：" + text)
		}
		return 0

	case WM_SIZE:
		cx := int32(uint16(lParam & 0xFFFF))
		cy := int32(uint16((lParam >> 16) & 0xFFFF))
		if hLabel != 0 {
			layoutControls(cx, cy)
		}
		return 0

	case WM_CLOSE:
		pDestroyWindow.Call(hwnd)
		return 0

	case WM_DESTROY:
		pPostQuitMessage.Call(0)
		return 0
	}

	ret, _, _ := pDefWindowProcW.Call(hwnd, msg, wParam, lParam)
	return ret
}

func cleanup() {
	if hFont != 0 {
		pDeleteObject.Call(hFont)
		hFont = 0
	}
	restoreW32IfStopped()
}

func createChild(hInst, parent uintptr, exStyle uintptr, class, text string, style uintptr, id uintptr, x, y, w, h int32) uintptr {
	var classPtr, textPtr uintptr
	if class != "" {
		classPtr = uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(class)))
	}
	if text != "" {
		textPtr = uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(text)))
	}
	hwnd, _, _ := pCreateWindowExW.Call(exStyle, classPtr, textPtr, style,
		uintptr(x), uintptr(y), uintptr(w), uintptr(h), parent, id, hInst, 0)
	return hwnd
}

func main() {
	runtime.LockOSThread()

	isElevated := false
	for _, arg := range os.Args[1:] {
		if arg == "-elevated" {
			isElevated = true
			break
		}
	}
	if !isElevated && !isRunningAsAdmin() {
		if err := selfElevate(); err != nil {
			showMsg(0, err.Error(), "错误", MB_OK|MB_ICONERROR)
		}
		os.Exit(0)
	}

	defer cleanup()

	dpi = systemDPI()

	hInst := getModuleHandle()
	if hInst == 0 {
		os.Exit(1)
	}

	className := syscall.StringToUTF16Ptr("SSTClass")
	windowTitle := syscall.StringToUTF16Ptr("设置系统时间")
	hCursor, _, _ := pLoadCursorW.Call(0, IDC_ARROW)

	wc := WNDCLASSEXW{
		Size:       uint32(unsafe.Sizeof(WNDCLASSEXW{})),
		WndProc:    syscall.NewCallback(wndProc),
		Instance:   hInst,
		Cursor:     hCursor,
		Background: COLOR_BTNFACE + 1,
		ClassName:  className,
	}
	atom, _, regErr := pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	if atom == 0 {
		if errno, ok := regErr.(syscall.Errno); !ok || errno != ERROR_CLASS_ALREADY_EXISTS {
			os.Exit(1)
		}
	}

	winW, winH := scale(baseWinW), scale(baseWinH)
	x, y := centeredPos(winW, winH)
	style := uintptr(WS_OVERLAPPED | WS_CAPTION | WS_SYSMENU | WS_MINIMIZEBOX | WS_MAXIMIZEBOX | WS_THICKFRAME)

	hwnd, _, _ := pCreateWindowExW.Call(0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(windowTitle)),
		style,
		uintptr(x), uintptr(y), uintptr(winW), uintptr(winH), 0, 0, hInst, 0)
	if hwnd == 0 {
		os.Exit(1)
	}
	hWndMain = hwnd

	fontH := -int32(scale(16)) // 负值表示字符单元高度，更清晰
	hFontRet, _, _ := pCreateFontW.Call(uintptr(uint32(fontH)), 0, 0, 0, 400, 0, 0, 0, 1, 0, 0, 0, 0,
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr("Microsoft YaHei"))))
	hFont = hFontRet

	m := int32(scale(16))
	gap := int32(scale(10))
	labelH := int32(scale(22))
	pickH := int32(scale(30))
	btnH := int32(scale(36))
	statusH := int32(scale(22))

	// 先建子控件（位置随后在 WM_SIZE / layoutControls 中校正）
	hLabel = createChild(hInst, hWndMain, 0, "STATIC", "选择日期：", WS_CHILD|WS_VISIBLE, ID_LABEL, m, m, 200, labelH)
	hDatePicker = createChild(hInst, hWndMain, WS_EX_CLIENTEDGE, "SysDateTimePick32", "", WS_CHILD|WS_VISIBLE, ID_DATEPICKER, m, m+labelH+gap, 300, pickH)

	btnW := int32(scale(140))
	btnY := m + labelH + gap + pickH + gap
	hBtnSet = createChild(hInst, hWndMain, 0, "BUTTON", "设置日期", WS_CHILD|WS_VISIBLE, ID_BTN_SET, m, btnY, btnW, btnH)
	hBtnRestore = createChild(hInst, hWndMain, 0, "BUTTON", "恢复时间", WS_CHILD|WS_VISIBLE, ID_BTN_RESTORE, m+btnW+scaleI32(16), btnY, btnW, btnH)

	logStyle := uintptr(WS_CHILD | WS_VISIBLE | WS_VSCROLL | ES_MULTILINE | ES_AUTOVSCROLL | ES_READONLY)
	hLog = createChild(hInst, hWndMain, WS_EX_CLIENTEDGE, "EDIT", "", logStyle, ID_LOG, m, btnY+btnH+gap, 300, scaleI32(120))
	hStatus = createChild(hInst, hWndMain, 0, "STATIC", "就绪", WS_CHILD|WS_VISIBLE, ID_STATUS, m, 300, 300, statusH)

	for _, h := range []uintptr{hLabel, hDatePicker, hBtnSet, hBtnRestore, hLog, hStatus} {
		pSendMessageW.Call(h, WM_SETFONT, hFont, 1)
	}

	// 触发一次布局（按当前客户区）
	var rc RECT
	pGetClientRect.Call(hWndMain, uintptr(unsafe.Pointer(&rc)))
	layoutControls(rc.Right-rc.Left, rc.Bottom-rc.Top)

	pShowWindow.Call(hWndMain, SW_SHOW)
	pUpdateWindow.Call(hWndMain)

	appendLog("就绪。选择日期后点击「设置日期」；点击「恢复时间」可同步 NTP。")

	var msg MSG
	for {
		ret, _, _ := pGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(ret) <= 0 {
			break
		}
		pTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		pDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}
}
