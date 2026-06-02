//go:build windows

package mh

import (
	"encoding/binary"
	"errors"
	"fmt"
	"syscall"
	"unsafe"
)

const errSemTimeout syscall.Errno = 121

var (
	procDebugActiveProcess        = modKernel32.NewProc("DebugActiveProcess")
	procDebugActiveProcessStop    = modKernel32.NewProc("DebugActiveProcessStop")
	procWaitForDebugEvent         = modKernel32.NewProc("WaitForDebugEvent")
	procContinueDebugEvent        = modKernel32.NewProc("ContinueDebugEvent")
	procDebugSetProcessKillOnExit = modKernel32.NewProc("DebugSetProcessKillOnExit")
)

const (
	EXCEPTION_DEBUG_EVENT      = 1
	CREATE_THREAD_DEBUG_EVENT  = 2
	CREATE_PROCESS_DEBUG_EVENT = 3
	EXIT_THREAD_DEBUG_EVENT    = 4
	EXIT_PROCESS_DEBUG_EVENT   = 5
	LOAD_DLL_DEBUG_EVENT       = 6
	UNLOAD_DLL_DEBUG_EVENT     = 7
	OUTPUT_DEBUG_STRING_EVENT  = 8
	RIP_EVENT                  = 9
)

const (
	DBG_CONTINUE              = 0x00010002
	DBG_EXCEPTION_NOT_HANDLED = 0x80010001
)

const (
	EXCEPTION_ACCESS_VIOLATION    = 0xC0000005
	EXCEPTION_BREAKPOINT          = 0x80000003
	EXCEPTION_SINGLE_STEP         = 0x80000004
	EXCEPTION_GUARD_PAGE          = 0x80000001
	EXCEPTION_ILLEGAL_INSTRUCTION = 0xC000001D
)

const INFINITE = 0xFFFFFFFF

const debugEventUnionSize = 160

type DEBUG_EVENT struct {
	DwDebugEventCode uint32
	DwProcessId      uint32
	DwThreadId       uint32
	_                uint32
	U                [debugEventUnionSize]byte
}

type EXCEPTION_RECORD struct {
	ExceptionCode        uint32
	ExceptionFlags       uint32
	ExceptionRecord      uintptr
	ExceptionAddress     uintptr
	NumberParameters     uint32
	_                    uint32
	ExceptionInformation [15]uintptr
}

func (e *DEBUG_EVENT) Exception() (rec EXCEPTION_RECORD, firstChance bool) {
	rec = *(*EXCEPTION_RECORD)(unsafe.Pointer(&e.U[0]))
	const exceptionRecordSize = unsafe.Sizeof(EXCEPTION_RECORD{})
	fc := binary.LittleEndian.Uint32(e.U[exceptionRecordSize : exceptionRecordSize+4])
	firstChance = fc != 0
	return
}

func DebugActiveProcess(pid uint32) error {
	ret, _, err := procDebugActiveProcess.Call(uintptr(pid))
	if ret == 0 {
		if IsErrSuccess(err) {
			return fmt.Errorf("DebugActiveProcess failed")
		}
		return err
	}
	return nil
}

func DebugActiveProcessStop(pid uint32) error {
	ret, _, err := procDebugActiveProcessStop.Call(uintptr(pid))
	if ret == 0 {
		if IsErrSuccess(err) {
			return fmt.Errorf("DebugActiveProcessStop failed")
		}
		return err
	}
	return nil
}

func DebugSetProcessKillOnExit(killOnExit bool) error {
	v := uintptr(0)
	if killOnExit {
		v = 1
	}
	ret, _, err := procDebugSetProcessKillOnExit.Call(v)
	if ret == 0 {
		if IsErrSuccess(err) {
			return fmt.Errorf("DebugSetProcessKillOnExit failed")
		}
		return err
	}
	return nil
}

func WaitForDebugEvent(evt *DEBUG_EVENT, timeoutMs uint32) (bool, error) {
	ret, _, err := procWaitForDebugEvent.Call(
		uintptr(unsafe.Pointer(evt)),
		uintptr(timeoutMs),
	)
	if ret == 0 {
		if IsErrSuccess(err) || errors.Is(err, errSemTimeout) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func ContinueDebugEvent(pid, tid, continueStatus uint32) error {
	ret, _, err := procContinueDebugEvent.Call(
		uintptr(pid),
		uintptr(tid),
		uintptr(continueStatus),
	)
	if ret == 0 {
		if IsErrSuccess(err) {
			return fmt.Errorf("ContinueDebugEvent failed")
		}
		return err
	}
	return nil
}
