package mh

import (
	"encoding/binary"
	"fmt"
	"syscall"
	"unsafe"
)

var (
	procThread32First     = modKernel32.NewProc("Thread32First")
	procThread32Next      = modKernel32.NewProc("Thread32Next")
	procOpenThread        = modKernel32.NewProc("OpenThread")
	procSuspendThread     = modKernel32.NewProc("SuspendThread")
	procResumeThread      = modKernel32.NewProc("ResumeThread")
	procGetThreadContext  = modKernel32.NewProc("GetThreadContext")
	procSetThreadContext  = modKernel32.NewProc("SetThreadContext")
	procGetCurrentThread  = modKernel32.NewProc("GetCurrentThread")
)

const TH32CS_SNAPTHREAD = 0x00000004

const (
	THREAD_GET_CONTEXT    = 0x0008
	THREAD_SET_CONTEXT    = 0x0010
	THREAD_SUSPEND_RESUME = 0x0002
	THREAD_QUERY_INFORMATION = 0x0040
	THREAD_ALL_ACCESS     = 0x1FFFFF
)

const (
	CONTEXT_AMD64           uint32 = 0x100000
	CONTEXT_CONTROL                = CONTEXT_AMD64 | 0x1
	CONTEXT_INTEGER                = CONTEXT_AMD64 | 0x2
	CONTEXT_SEGMENTS               = CONTEXT_AMD64 | 0x4
	CONTEXT_FLOATING_POINT         = CONTEXT_AMD64 | 0x8
	CONTEXT_DEBUG_REGISTERS        = CONTEXT_AMD64 | 0x10
	CONTEXT_FULL                   = CONTEXT_AMD64 | 0x1 | 0x2 | 0x4
	CONTEXT_ALL                    = CONTEXT_AMD64 | 0x1F
)

const contextAMD64Size = 1232

const (
	ctxOffContextFlags = 0x30
	ctxOffDr0          = 0x48
	ctxOffDr1          = 0x50
	ctxOffDr2          = 0x58
	ctxOffDr3          = 0x60
	ctxOffDr6          = 0x68
	ctxOffDr7          = 0x70
	ctxOffRax          = 0x78
	ctxOffRcx          = 0x80
	ctxOffRdx          = 0x88
	ctxOffRbx          = 0x90
	ctxOffRsp          = 0x98
	ctxOffRbp          = 0xA0
	ctxOffRsi          = 0xA8
	ctxOffRdi          = 0xB0
	ctxOffR8           = 0xB8
	ctxOffR9           = 0xC0
	ctxOffR10          = 0xC8
	ctxOffR11          = 0xD0
	ctxOffR12          = 0xD8
	ctxOffR13          = 0xE0
	ctxOffR14          = 0xE8
	ctxOffR15          = 0xF0
	ctxOffRip          = 0xF8
	ctxOffXmm0         = 0x1A0
)

type THREADENTRY32 struct {
	Size           uint32
	CntUsage       uint32
	ThreadID       uint32
	OwnerProcessID uint32
	BasePri        int32
	DeltaPri       int32
	Flags          uint32
}

type Context struct {
	buf []byte
}

func NewContext() *Context {
	raw := make([]byte, contextAMD64Size+15)
	addr := uintptr(unsafe.Pointer(&raw[0]))
	off := (16 - addr&15) & 15
	return &Context{buf: raw[off : off+contextAMD64Size]}
}

func (c *Context) ptr() unsafe.Pointer { return unsafe.Pointer(&c.buf[0]) }

func (c *Context) Bytes() []byte { return c.buf }

func (c *Context) SetContextFlags(v uint32) {
	binary.LittleEndian.PutUint32(c.buf[ctxOffContextFlags:], v)
}

func (c *Context) ContextFlags() uint32 {
	return binary.LittleEndian.Uint32(c.buf[ctxOffContextFlags:])
}

func (c *Context) getU64(off int) uint64 { return binary.LittleEndian.Uint64(c.buf[off:]) }
func (c *Context) setU64(off int, v uint64) {
	binary.LittleEndian.PutUint64(c.buf[off:], v)
}

func (c *Context) SetDr0(v uint64) { c.setU64(ctxOffDr0, v) }
func (c *Context) Dr0() uint64     { return c.getU64(ctxOffDr0) }
func (c *Context) SetDr7(v uint64) { c.setU64(ctxOffDr7, v) }
func (c *Context) Dr7() uint64     { return c.getU64(ctxOffDr7) }
func (c *Context) Dr6() uint64     { return c.getU64(ctxOffDr6) }

func (c *Context) Rip() uint64 { return c.getU64(ctxOffRip) }
func (c *Context) Rax() uint64 { return c.getU64(ctxOffRax) }
func (c *Context) Rcx() uint64 { return c.getU64(ctxOffRcx) }
func (c *Context) Rdx() uint64 { return c.getU64(ctxOffRdx) }
func (c *Context) Rbx() uint64 { return c.getU64(ctxOffRbx) }
func (c *Context) Rsp() uint64 { return c.getU64(ctxOffRsp) }
func (c *Context) Rbp() uint64 { return c.getU64(ctxOffRbp) }
func (c *Context) Rsi() uint64 { return c.getU64(ctxOffRsi) }
func (c *Context) Rdi() uint64 { return c.getU64(ctxOffRdi) }
func (c *Context) R8() uint64  { return c.getU64(ctxOffR8) }
func (c *Context) R9() uint64  { return c.getU64(ctxOffR9) }
func (c *Context) R10() uint64 { return c.getU64(ctxOffR10) }
func (c *Context) R11() uint64 { return c.getU64(ctxOffR11) }
func (c *Context) R12() uint64 { return c.getU64(ctxOffR12) }
func (c *Context) R13() uint64 { return c.getU64(ctxOffR13) }
func (c *Context) R14() uint64 { return c.getU64(ctxOffR14) }
func (c *Context) R15() uint64 { return c.getU64(ctxOffR15) }

func (c *Context) GPR(idx int) uint64 {
	switch idx {
	case 0:
		return c.Rax()
	case 1:
		return c.Rcx()
	case 2:
		return c.Rdx()
	case 3:
		return c.Rbx()
	case 4:
		return c.Rsp()
	case 5:
		return c.Rbp()
	case 6:
		return c.Rsi()
	case 7:
		return c.Rdi()
	case 8:
		return c.R8()
	case 9:
		return c.R9()
	case 10:
		return c.R10()
	case 11:
		return c.R11()
	case 12:
		return c.R12()
	case 13:
		return c.R13()
	case 14:
		return c.R14()
	case 15:
		return c.R15()
	}
	return 0
}

func GPRName(idx int) string {
	names := [16]string{"rax", "rcx", "rdx", "rbx", "rsp", "rbp", "rsi", "rdi", "r8", "r9", "r10", "r11", "r12", "r13", "r14", "r15"}
	if idx < 0 || idx >= 16 {
		return fmt.Sprintf("r?%d", idx)
	}
	return names[idx]
}

func Thread32First(snapshot HANDLE, te *THREADENTRY32) bool {
	if te.Size == 0 {
		te.Size = uint32(unsafe.Sizeof(*te))
	}
	ret, _, _ := procThread32First.Call(uintptr(snapshot), uintptr(unsafe.Pointer(te)))
	return ret != 0
}

func Thread32Next(snapshot HANDLE, te *THREADENTRY32) bool {
	if te.Size == 0 {
		te.Size = uint32(unsafe.Sizeof(*te))
	}
	ret, _, _ := procThread32Next.Call(uintptr(snapshot), uintptr(unsafe.Pointer(te)))
	return ret != 0
}

func OpenThread(desiredAccess uint32, inheritHandle bool, tid uint32) (HANDLE, error) {
	inherit := uintptr(0)
	if inheritHandle {
		inherit = 1
	}
	ret, _, err := procOpenThread.Call(uintptr(desiredAccess), inherit, uintptr(tid))
	if ret == 0 {
		if IsErrSuccess(err) {
			return 0, fmt.Errorf("OpenThread failed for tid %d", tid)
		}
		return 0, err
	}
	return HANDLE(ret), nil
}

func SuspendThread(h HANDLE) (uint32, error) {
	ret, _, err := procSuspendThread.Call(uintptr(h))
	if int32(ret) < 0 {
		if IsErrSuccess(err) {
			return 0, fmt.Errorf("SuspendThread failed")
		}
		return 0, err
	}
	return uint32(ret), nil
}

func ResumeThread(h HANDLE) (uint32, error) {
	ret, _, err := procResumeThread.Call(uintptr(h))
	if int32(ret) < 0 {
		if IsErrSuccess(err) {
			return 0, fmt.Errorf("ResumeThread failed")
		}
		return 0, err
	}
	return uint32(ret), nil
}

func GetThreadContext(h HANDLE, ctx *Context) error {
	ret, _, err := procGetThreadContext.Call(uintptr(h), uintptr(ctx.ptr()))
	if ret == 0 {
		if IsErrSuccess(err) {
			return fmt.Errorf("GetThreadContext failed")
		}
		return err
	}
	return nil
}

func SetThreadContext(h HANDLE, ctx *Context) error {
	ret, _, err := procSetThreadContext.Call(uintptr(h), uintptr(ctx.ptr()))
	if ret == 0 {
		if IsErrSuccess(err) {
			return fmt.Errorf("SetThreadContext failed")
		}
		return err
	}
	return nil
}

func GetCurrentThread() HANDLE {
	ret, _, _ := procGetCurrentThread.Call()
	return HANDLE(ret)
}

func EnumerateThreads(pid uint32) ([]uint32, error) {
	snap, err := CreateToolhelp32Snapshot(TH32CS_SNAPTHREAD, 0)
	if err != nil {
		return nil, fmt.Errorf("snapshot threads: %w", err)
	}
	defer CloseHandle(snap)

	var tids []uint32
	var te THREADENTRY32
	if !Thread32First(snap, &te) {
		return nil, fmt.Errorf("Thread32First failed")
	}
	for {
		if te.OwnerProcessID == pid {
			tids = append(tids, te.ThreadID)
		}
		if !Thread32Next(snap, &te) {
			break
		}
	}
	return tids, nil
}

var _ syscall.Errno
