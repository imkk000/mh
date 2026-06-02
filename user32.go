//go:build windows

package mh

import (
	"fmt"
	"syscall"
	"unsafe"
)

var (
	modUser32                    = syscall.NewLazyDLL("user32.dll")
	procFindWindow               = modUser32.NewProc("FindWindowExA")
	procPostMessage              = modUser32.NewProc("PostMessageA")
	procGetForegroundWindow      = modUser32.NewProc("GetForegroundWindow")
	procGetWindowText            = modUser32.NewProc("GetWindowTextA")
	procGetWindowRect            = modUser32.NewProc("GetWindowRect")
	procGetClientRect            = modUser32.NewProc("GetClientRect")
	procClientToScreen           = modUser32.NewProc("ClientToScreen")
	procSendInput                = modUser32.NewProc("SendInput")
	procGetCursorPos             = modUser32.NewProc("GetCursorPos")
	procSetCursorPos             = modUser32.NewProc("SetCursorPos")
	procEnumWindows              = modUser32.NewProc("EnumWindows")
	procGetWindowThreadProcessId = modUser32.NewProc("GetWindowThreadProcessId")
	procGetClassName             = modUser32.NewProc("GetClassNameW")
	procGetAsyncKeyState         = modUser32.NewProc("GetAsyncKeyState")
	procMoveWindow               = modUser32.NewProc("MoveWindow")
)

func MoveWindow(hwnd HWND, x, y, width, height int, repaint bool) error {
	rp := uintptr(0)
	if repaint {
		rp = 1
	}
	ret, _, err := procMoveWindow.Call(uintptr(hwnd), uintptr(x), uintptr(y), uintptr(width), uintptr(height), rp)
	if ret == 0 {
		if IsErrSuccess(err) {
			return fmt.Errorf("MoveWindow failed")
		}
		return err
	}
	return nil
}

func PostMessage(hwnd HWND, msg uint32, wParam uint32, lParam ...int64) error {
	var lp int64
	if lParam != nil {
		lp = lParam[0]
	}
	ret, _, err := procPostMessage.Call(uintptr(hwnd), uintptr(msg), uintptr(wParam), uintptr(lp))
	if ret == 0 {
		if IsErrSuccess(err) {
			return fmt.Errorf("PostMessage failed")
		}
		return err
	}
	return nil
}

func FindWindow(className, windowName string) (HWND, error) {
	lpszClass := []byte(className)
	lpszWindow := []byte(windowName)
	ret, _, err := procFindWindow.Call(0, 0, uintptr(unsafe.Pointer(&lpszClass[0])), uintptr(unsafe.Pointer(&lpszWindow[0])))
	if ret == 0 {
		if IsErrSuccess(err) {
			return 0, fmt.Errorf("FindWindow: %q %q not found", className, windowName)
		}
		return 0, err
	}
	return HWND(ret), nil
}

func EnumWindows(lpEnumFunc WNDENUMPROC) error {
	fn := func(hwnd HWND, _ uintptr) int {
		lpEnumFunc(hwnd)
		return 1
	}
	ret, _, err := procEnumWindows.Call(syscall.NewCallback(fn), 0)
	if ret == 0 {
		if IsErrSuccess(err) {
			return fmt.Errorf("EnumWindows failed")
		}
		return err
	}
	return nil
}

func GetWindowThreadProcessId(hwnd HWND) (HANDLE, int) {
	var processId int
	ret, _, _ := procGetWindowThreadProcessId.Call(
		uintptr(hwnd),
		uintptr(unsafe.Pointer(&processId)),
	)

	return HANDLE(ret), processId
}

const maxClassName = 256

func GetClassNameW(hwnd HWND) string {
	buf := make([]uint16, maxClassName)
	procGetClassName.Call(
		uintptr(hwnd),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(maxClassName),
	)

	return syscall.UTF16ToString(buf)
}

type (
	WNDENUMPROC func(hwnd HWND)
	HWND        uintptr
)

const (
	WM_KEYDOWN = 0x0100
	WM_KEYUP   = 0x0101
	WM_CHAR    = 0x0102
)

func GetForegroundWindow() HWND {
	ret, _, _ := procGetForegroundWindow.Call()
	return HWND(ret)
}

func GetWindowText(hwnd HWND) string {
	buf := make([]byte, MAX_PATH)
	ret, _, _ := procGetWindowText.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&buf[0])), MAX_PATH)
	return string(buf[:ret])
}

func GetWindowRect(hwnd HWND) (Rect, error) {
	var rect Rect
	ret, _, err := procGetWindowRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&rect)))
	if ret == 0 {
		if IsErrSuccess(err) {
			return rect, fmt.Errorf("GetWindowRect failed")
		}
		return rect, err
	}
	return rect, nil
}

func GetClientRect(hwnd HWND) (Rect, error) {
	var rect Rect
	ret, _, err := procGetClientRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&rect)))
	if ret == 0 {
		if IsErrSuccess(err) {
			return rect, fmt.Errorf("GetClientRect failed")
		}
		return rect, err
	}
	return rect, nil
}

func ClientToScreen(hwnd HWND) (Point, error) {
	var point Point
	ret, _, err := procClientToScreen.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&point)))
	if ret == 0 {
		if IsErrSuccess(err) {
			return point, fmt.Errorf("ClientToScreen failed")
		}
		return point, err
	}
	return point, nil
}

type Rect struct {
	Left, Top, Right, Bottom int32
}

type Point struct {
	X, Y int32
}

// Input mirrors the Win32 INPUT structure (40 bytes on x64).
// Construct with NewMouseInput or NewKeyboardInput.
type Input struct {
	Type uint32
	_    uint32   // align union on 8-byte boundary
	data [32]byte // sized for MOUSEINPUT (the larger of the two unions used)
}

type MouseInput struct {
	Dx          int32
	Dy          int32
	MouseData   uint32
	DwFlags     uint32
	Time        uint32
	DwExtraInfo uintptr
}

type KeyboardInput struct {
	WVk         uint16
	WScan       uint16
	DwFlags     uint32
	Time        uint32
	DwExtraInfo uintptr
}

const (
	INPUT_MOUSE    = 0
	INPUT_KEYBOARD = 1
)

const (
	MOUSEEVENTF_LEFTDOWN = 0x0002
	MOUSEEVENTF_LEFTUP   = 0x0004
)

func NewMouseInput(mi MouseInput) Input {
	var in Input
	in.Type = INPUT_MOUSE
	*(*MouseInput)(unsafe.Pointer(&in.data[0])) = mi
	return in
}

func NewKeyboardInput(ki KeyboardInput) Input {
	var in Input
	in.Type = INPUT_KEYBOARD
	*(*KeyboardInput)(unsafe.Pointer(&in.data[0])) = ki
	return in
}

func SendInput(inputs []Input) error {
	if len(inputs) == 0 {
		return nil
	}
	ret, _, err := procSendInput.Call(
		uintptr(len(inputs)),
		uintptr(unsafe.Pointer(&inputs[0])),
		unsafe.Sizeof(inputs[0]),
	)
	if ret != uintptr(len(inputs)) {
		if IsErrSuccess(err) {
			return fmt.Errorf("SendInput: %d of %d events sent", ret, len(inputs))
		}
		return err
	}
	return nil
}

func GetCursorPos() (int32, int32, error) {
	var p Point
	ret, _, err := procGetCursorPos.Call(uintptr(unsafe.Pointer(&p)))
	if ret == 0 {
		if IsErrSuccess(err) {
			return 0, 0, fmt.Errorf("GetCursorPos failed")
		}
		return 0, 0, err
	}
	return p.X, p.Y, nil
}

func SetCursorPos(x, y int) error {
	ret, _, err := procSetCursorPos.Call(uintptr(x), uintptr(y))
	if ret == 0 {
		if IsErrSuccess(err) {
			return fmt.Errorf("SetCursorPos failed")
		}
		return err
	}
	return nil
}

func GetAsyncKeyState(vKey int) uint16 {
	ret, _, _ := procGetAsyncKeyState.Call(uintptr(vKey))
	return uint16(ret)
}

const (
	VK_LBUTTON  = 0x01
	VK_RBUTTON  = 0x02
	VK_MBUTTON  = 0x04
	VK_XBUTTON1 = 0x05
	VK_XBUTTON2 = 0x06
	VK_SPACE    = 0x20
	VK_F2       = 0x71
	VK_F3       = 0x72
	VK_F4       = 0x73
)
