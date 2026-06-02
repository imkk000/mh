package mh

import "errors"

type Process interface {
	PID() uint32
	ModuleBase() uint64
	Close() error

	Read(addr uint64, n int) ([]byte, error)
	ReadU8(addr uint64) (byte, error)
	ReadU32(addr uint64) (uint32, error)
	ReadU64(addr uint64) (uint64, error)
	ReadFloat32(addr uint64) (float32, error)
	ReadOffsets(base uint64, offsets ...uint64) uint64

	WriteU8(addr uint64, v byte) error
	WriteFloat32(addr uint64, v float32) error

	ReadableRegions() ([]Region, error)

	IsForeground() bool
	SendMouseClick(btn MouseButton) error
	SendChar(r rune) error
	IsKeyDown(k Key) bool
}

type Region struct {
	Start, End uint64
	Executable bool
	Writable   bool
}

type MouseButton uint8

const (
	MouseLeft MouseButton = iota + 1
	MouseRight
	MouseMiddle
)

type Key uint16

const (
	KeyLeftMouse Key = iota + 1
	KeyRightMouse
	KeyMiddleMouse
	KeyXButton1
	KeyXButton2
	KeySpace
	KeyF2
	KeyF3
	KeyF4
)

var ErrShortRead = errors.New("mh: short read")

func IsRunning(processName string) bool {
	return isRunning(processName)
}
