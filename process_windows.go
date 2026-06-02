//go:build windows

package mh

import (
	"encoding/binary"
	"fmt"
	"math"
)

type winProcess struct {
	pid       uint32
	baseAddr  uint64
	handle    HANDLE
	hwnd      HWND
	className string
}

func Attach(processName string) (Process, error) {
	return AttachWithClass(processName, "POEWindowClass")
}

func isRunning(processName string) bool {
	_, err := GetPID(processName)
	return err == nil
}

func AttachWithClass(processName, className string) (Process, error) {
	pid, err := GetPID(processName)
	if err != nil {
		return nil, err
	}
	h, err := OpenProcess(PROCESS_ALL_ACCESS, false, pid)
	if err != nil {
		return nil, fmt.Errorf("open process: %w", err)
	}
	base, err := FindModuleBase(pid, processName)
	if err != nil {
		CloseHandle(h)
		return nil, fmt.Errorf("module base: %w", err)
	}
	var hwnd HWND
	if err := EnumWindows(func(w HWND) {
		_, id := GetWindowThreadProcessId(w)
		if uint32(id) != pid {
			return
		}
		if className != "" && GetClassNameW(w) != className {
			return
		}
		hwnd = w
	}); err != nil {
		CloseHandle(h)
		return nil, fmt.Errorf("enum windows: %w", err)
	}
	return &winProcess{pid: pid, baseAddr: base, handle: h, hwnd: hwnd, className: className}, nil
}

func (p *winProcess) PID() uint32        { return p.pid }
func (p *winProcess) ModuleBase() uint64 { return p.baseAddr }
func (p *winProcess) Close() error {
	if p.handle != 0 {
		CloseHandle(p.handle)
		p.handle = 0
	}
	return nil
}

func (p *winProcess) Read(addr uint64, n int) ([]byte, error) {
	return ReadProcessMemory(p.handle, addr, uint(n))
}

func (p *winProcess) ReadU8(addr uint64) (byte, error) {
	data, err := ReadProcessMemory(p.handle, addr, 1)
	if err != nil || len(data) < 1 {
		return 0, err
	}
	return data[0], nil
}

func (p *winProcess) ReadU32(addr uint64) (uint32, error) {
	data, err := ReadProcessMemory(p.handle, addr, 4)
	if err != nil || len(data) < 4 {
		return 0, err
	}
	return binary.LittleEndian.Uint32(data), nil
}

func (p *winProcess) ReadU64(addr uint64) (uint64, error) {
	data, err := ReadProcessMemory(p.handle, addr, 8)
	if err != nil || len(data) < 8 {
		return 0, err
	}
	return binary.LittleEndian.Uint64(data), nil
}

func (p *winProcess) ReadFloat32(addr uint64) (float32, error) {
	data, err := ReadProcessMemory(p.handle, addr, 4)
	if err != nil || len(data) < 4 {
		return 0, err
	}
	return math.Float32frombits(binary.LittleEndian.Uint32(data)), nil
}

func (p *winProcess) ReadOffsets(base uint64, offsets ...uint64) uint64 {
	addr := base
	for _, off := range offsets {
		v, err := p.ReadU64(addr + off)
		if err != nil || v == 0 {
			return 0
		}
		addr = v
	}
	return addr
}

func (p *winProcess) WriteU8(addr uint64, v byte) error {
	return WriteProcessMemory(p.handle, addr, []byte{v}, 1)
}

func (p *winProcess) WriteFloat32(addr uint64, v float32) error {
	data := binary.LittleEndian.AppendUint32(nil, math.Float32bits(v))
	return WriteProcessMemory(p.handle, addr, data, 4)
}

func (p *winProcess) ReadableRegions() ([]Region, error) {
	var regions []Region
	var addr uintptr
	for {
		mbi, ok := VirtualQueryEx(p.handle, addr)
		if !ok {
			break
		}
		end := mbi.BaseAddress + mbi.RegionSize
		if end <= addr {
			break
		}
		addr = end
		if mbi.State != MEM_COMMIT {
			continue
		}
		exec := false
		write := false
		switch mbi.Protect {
		case PAGE_EXECUTE, PAGE_EXECUTE_READ:
			exec = true
		case PAGE_EXECUTE_READWRITE:
			exec = true
			write = true
		case PAGE_READWRITE:
			write = true
		case PAGE_READONLY:
		default:
			continue
		}
		regions = append(regions, Region{
			Start:      uint64(mbi.BaseAddress),
			End:        uint64(end),
			Executable: exec,
			Writable:   write,
		})
	}
	return regions, nil
}

func (p *winProcess) IsForeground() bool {
	if p.hwnd == 0 {
		return true
	}
	return p.hwnd == GetForegroundWindow()
}

func (p *winProcess) SendMouseClick(btn MouseButton) error {
	down, up, err := mouseFlags(btn)
	if err != nil {
		return err
	}
	return SendInput([]Input{
		NewMouseInput(MouseInput{DwFlags: down}),
		NewMouseInput(MouseInput{DwFlags: up}),
	})
}

func mouseFlags(btn MouseButton) (down, up uint32, err error) {
	const (
		MOUSEEVENTF_RIGHTDOWN  = 0x0008
		MOUSEEVENTF_RIGHTUP    = 0x0010
		MOUSEEVENTF_MIDDLEDOWN = 0x0020
		MOUSEEVENTF_MIDDLEUP   = 0x0040
	)
	switch btn {
	case MouseLeft:
		return MOUSEEVENTF_LEFTDOWN, MOUSEEVENTF_LEFTUP, nil
	case MouseRight:
		return MOUSEEVENTF_RIGHTDOWN, MOUSEEVENTF_RIGHTUP, nil
	case MouseMiddle:
		return MOUSEEVENTF_MIDDLEDOWN, MOUSEEVENTF_MIDDLEUP, nil
	}
	return 0, 0, fmt.Errorf("mh: unknown mouse button %d", btn)
}

func (p *winProcess) SendChar(r rune) error {
	if p.hwnd == 0 {
		return fmt.Errorf("mh: no target window for SendChar")
	}
	return PostMessageAsRune(p.hwnd, WM_KEYDOWN, r)
}

func (p *winProcess) IsKeyDown(k Key) bool {
	vk, ok := vkFromKey(k)
	if !ok {
		return false
	}
	return GetAsyncKeyState(vk) != 0
}

func vkFromKey(k Key) (int, bool) {
	switch k {
	case KeyLeftMouse:
		return VK_LBUTTON, true
	case KeyRightMouse:
		return VK_RBUTTON, true
	case KeyMiddleMouse:
		return VK_MBUTTON, true
	case KeyXButton1:
		return VK_XBUTTON1, true
	case KeyXButton2:
		return VK_XBUTTON2, true
	case KeySpace:
		return VK_SPACE, true
	case KeyF2:
		return VK_F2, true
	case KeyF3:
		return VK_F3, true
	case KeyF4:
		return VK_F4, true
	}
	return 0, false
}
