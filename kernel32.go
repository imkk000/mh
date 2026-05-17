package mh

import (
	"fmt"
	"log"
	"os"
	"syscall"
	"unsafe"
)

var (
	modKernel32                  = syscall.NewLazyDLL("kernel32.dll")
	procAllocConsole             = modKernel32.NewProc("AllocConsole")
	procFreeConsole              = modKernel32.NewProc("FreeConsole")
	procSetConsoleTitle          = modKernel32.NewProc("SetConsoleTitleW")
	procGetCurrentProcess        = modKernel32.NewProc("GetCurrentProcess")
	procReadProcessMemory        = modKernel32.NewProc("ReadProcessMemory")
	procWriteProcessMemory       = modKernel32.NewProc("WriteProcessMemory")
	procModule32First            = modKernel32.NewProc("Module32FirstW")
	procModule32Next             = modKernel32.NewProc("Module32NextW")
	procCreateToolhelp32Snapshot = modKernel32.NewProc("CreateToolhelp32Snapshot")
	procCloseHandle              = modKernel32.NewProc("CloseHandle")
	procProcess32First           = modKernel32.NewProc("Process32FirstW")
	procProcess32Next            = modKernel32.NewProc("Process32NextW")
	procOpenProcess              = modKernel32.NewProc("OpenProcess")
	procVirtualQueryEx           = modKernel32.NewProc("VirtualQueryEx")
	procGetConsoleWindow         = modKernel32.NewProc("GetConsoleWindow")
	procVirtualAllocEx           = modKernel32.NewProc("VirtualAllocEx")
	procVirtualFreeEx            = modKernel32.NewProc("VirtualFreeEx")
)

func VirtualAllocEx(h HANDLE, addr uint64, size uint64, allocType, protect uint32) (uint64, error) {
	ret, _, err := procVirtualAllocEx.Call(
		uintptr(h),
		uintptr(addr),
		uintptr(size),
		uintptr(allocType),
		uintptr(protect),
	)
	if ret == 0 {
		if IsErrSuccess(err) {
			return 0, fmt.Errorf("VirtualAllocEx failed")
		}
		return 0, err
	}
	return uint64(ret), nil
}

func VirtualFreeEx(h HANDLE, addr uint64, size uint64, freeType uint32) error {
	ret, _, err := procVirtualFreeEx.Call(
		uintptr(h),
		uintptr(addr),
		uintptr(size),
		uintptr(freeType),
	)
	if ret == 0 {
		if IsErrSuccess(err) {
			return fmt.Errorf("VirtualFreeEx failed")
		}
		return err
	}
	return nil
}

func GetConsoleWindow() HWND {
	ret, _, _ := procGetConsoleWindow.Call()
	return HWND(ret)
}

func OpenProcess(desiredAccess uint32, inheritHandle bool, processId uint32) (HANDLE, error) {
	inherit := uintptr(0)
	if inheritHandle {
		inherit = 1
	}
	ret, _, err := procOpenProcess.Call(
		uintptr(desiredAccess),
		inherit,
		uintptr(processId),
	)
	if !IsErrSuccess(err) {
		return 0, err
	}
	return HANDLE(ret), nil
}

func ReadProcessMemoryAs[T any](hProcess HANDLE, lpBaseAddress uint64) (v T, err error) {
	var numBytesRead uintptr
	size := unsafe.Sizeof(v)
	_, _, err = procReadProcessMemory.Call(uintptr(hProcess),
		uintptr(lpBaseAddress),
		uintptr(unsafe.Pointer(&v)),
		uintptr(size),
		uintptr(unsafe.Pointer(&numBytesRead)))
	if !IsErrSuccess(err) {
		return
	}
	err = nil
	return
}

func ReadProcessMemory(hProcess HANDLE, lpBaseAddress uint64, size uint) ([]byte, error) {
	data := make([]byte, size)
	var numBytesRead uintptr
	_, _, err := procReadProcessMemory.Call(uintptr(hProcess),
		uintptr(lpBaseAddress),
		uintptr(unsafe.Pointer(&data[0])),
		uintptr(size),
		uintptr(unsafe.Pointer(&numBytesRead)))
	if !IsErrSuccess(err) {
		return nil, err
	}
	if numBytesRead != uintptr(size) {
		return data[:numBytesRead], fmt.Errorf("short read at %X: %d of %d bytes", lpBaseAddress, numBytesRead, size)
	}
	return data, nil
}

func WriteProcessMemory(hProcess HANDLE, lpBaseAddress uint64, data []byte, size uint) error {
	var numBytesWritten uintptr
	_, _, err := procWriteProcessMemory.Call(uintptr(hProcess),
		uintptr(lpBaseAddress),
		uintptr(unsafe.Pointer(&data[0])),
		uintptr(size),
		uintptr(unsafe.Pointer(&numBytesWritten)))
	if !IsErrSuccess(err) {
		return err
	}
	if numBytesWritten != uintptr(size) {
		return fmt.Errorf("short write at %X: %d of %d bytes", lpBaseAddress, numBytesWritten, size)
	}
	return nil
}

func Module32First(snapshot HANDLE, me *MODULEENTRY32) bool {
	if me.Size == 0 {
		me.Size = uint32(unsafe.Sizeof(*me))
	}
	ret, _, _ := procModule32First.Call(
		uintptr(snapshot),
		uintptr(unsafe.Pointer(me)),
	)
	return ret != 0
}

func Module32Next(snapshot HANDLE, me *MODULEENTRY32) bool {
	if me.Size == 0 {
		me.Size = uint32(unsafe.Sizeof(*me))
	}
	ret, _, _ := procModule32Next.Call(
		uintptr(snapshot),
		uintptr(unsafe.Pointer(me)),
	)
	return ret != 0
}

func CreateToolhelp32Snapshot(flags, processId uint32) (HANDLE, error) {
	ret, _, err := procCreateToolhelp32Snapshot.Call(
		uintptr(flags),
		uintptr(processId),
	)
	if ret == 0 || ret == ^uintptr(0) {
		if IsErrSuccess(err) {
			return 0, fmt.Errorf("CreateToolhelp32Snapshot failed")
		}
		return 0, err
	}
	return HANDLE(ret), nil
}

func CloseHandle(object HANDLE) bool {
	ret, _, _ := procCloseHandle.Call(
		uintptr(object),
	)
	return ret != 0
}

func GetCurrentProcess() HANDLE {
	ret, _, _ := procGetCurrentProcess.Call()
	return HANDLE(ret)
}

func FreeConsole() error {
	os.Stdout = nil
	log.SetOutput(nil)
	ret, _, err := procFreeConsole.Call()
	if ret == 0 {
		if IsErrSuccess(err) {
			return fmt.Errorf("FreeConsole failed")
		}
		return err
	}
	return nil
}

func AllocConsole() error {
	ret, _, err := procAllocConsole.Call()
	if ret == 0 {
		if IsErrSuccess(err) {
			return fmt.Errorf("AllocConsole failed")
		}
		return err
	}
	os.Stdout, _ = os.OpenFile("CONOUT$", os.O_WRONLY, 0o666)
	log.SetOutput(os.Stdout)
	return nil
}

func SetConsoleTitle(title string) error {
	ptr, err := syscall.UTF16PtrFromString(title)
	if err != nil {
		return err
	}
	ret, _, callErr := procSetConsoleTitle.Call(uintptr(unsafe.Pointer(ptr)))
	if ret == 0 {
		if IsErrSuccess(callErr) {
			return fmt.Errorf("SetConsoleTitle failed")
		}
		return callErr
	}
	return nil
}

func IsErrSuccess(err error) bool {
	if errno, ok := err.(syscall.Errno); ok {
		if errno == 0 {
			return true
		}
	}
	return false
}

func Process32First(snapshot HANDLE, pe *PROCESSENTRY32) bool {
	if pe.Size == 0 {
		pe.Size = uint32(unsafe.Sizeof(*pe))
	}
	ret, _, _ := procProcess32First.Call(
		uintptr(snapshot),
		uintptr(unsafe.Pointer(pe)),
	)

	return ret != 0
}

func Process32Next(snapshot HANDLE, pe *PROCESSENTRY32) bool {
	if pe.Size == 0 {
		pe.Size = uint32(unsafe.Sizeof(*pe))
	}
	ret, _, _ := procProcess32Next.Call(
		uintptr(snapshot),
		uintptr(unsafe.Pointer(pe)),
	)

	return ret != 0
}

type MEMORY_BASIC_INFORMATION struct {
	BaseAddress       uintptr
	AllocationBase    uintptr
	AllocationProtect uint32
	RegionSize        uintptr
	State             uint32
	Protect           uint32
	Type              uint32
}

func VirtualQueryEx(hProcess HANDLE, lpAddress uintptr) (MEMORY_BASIC_INFORMATION, bool) {
	var result MEMORY_BASIC_INFORMATION
	ret, _, _ := procVirtualQueryEx.Call(
		uintptr(hProcess),
		lpAddress,
		uintptr(unsafe.Pointer(&result)),
		unsafe.Sizeof(result),
	)
	return result, ret != 0
}

type MODULEENTRY32 struct {
	Size         uint32
	ModuleID     uint32
	ProcessID    uint32
	GlblcntUsage uint32
	ProccntUsage uint32
	ModBaseAddr  uintptr
	ModBaseSize  uint32
	HModule      uintptr
	SzModule     [255 + 1]uint16
	SzExePath    [260]uint16
}

type PROCESSENTRY32 struct {
	Size            uint32
	CntUsage        uint32
	ProcessID       uint32
	DefaultHeapID   uintptr
	ModuleID        uint32
	Threads         uint32
	ParentProcessID uint32
	PriClassBase    int32
	Flags           uint32
	ExeFile         [260]uint16
}

type HANDLE uintptr

const (
	TH32CS_SNAPMODULE   = 0x00000008
	TH32CS_SNAPMODULE32 = 0x00000010
	TH32CS_SNAPPROCESS  = 0x00000002
)

const (
	PROCESS_ALL_ACCESS = 2035711
)

const (
	MEM_COMMIT  = 0x1000
	MEM_RESERVE = 0x2000
	MEM_RELEASE = 0x8000
)

const (
	PAGE_NOACCESS          = 0x01
	PAGE_READONLY          = 0x02
	PAGE_READWRITE         = 0x04
	PAGE_EXECUTE           = 0x10
	PAGE_EXECUTE_READ      = 0x20
	PAGE_EXECUTE_READWRITE = 0x40
)

const MAX_PATH = 260
