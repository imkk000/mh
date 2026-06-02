//go:build windows

package mh

import (
	"encoding/binary"
	"math"
)

func ReadOffsets(h HANDLE, baseAddr uint64, offset ...uint64) uint64 {
	addr := baseAddr
	for _, offset := range offset {
		addr = ReadProcessMemoryAsUint64(h, addr+offset)
		if addr == 0 {
			return 0
		}
	}
	return addr
}

func ReadProcessMemoryAsByte(h HANDLE, addr uint64) byte {
	data, _ := ReadProcessMemory(h, addr, 1)
	if len(data) < 1 {
		return 0
	}
	return data[0]
}

func ReadProcessMemoryAsUint32(h HANDLE, addr uint64) uint32 {
	data, _ := ReadProcessMemory(h, addr, 4)
	if len(data) < 4 {
		return 0
	}
	return binary.LittleEndian.Uint32(data)
}

func ReadProcessMemoryAsUint64(h HANDLE, addr uint64) uint64 {
	data, _ := ReadProcessMemory(h, addr, 8)
	if len(data) < 8 {
		return 0
	}
	return binary.LittleEndian.Uint64(data)
}

func ReadProcessMemoryAsFloat32(h HANDLE, addr uint64) float32 {
	data, _ := ReadProcessMemory(h, addr, 4)
	if len(data) < 4 {
		return 0
	}
	bits := binary.LittleEndian.Uint32(data)
	return math.Float32frombits(bits)
}

func WriteProcessMemoryAsByte(h HANDLE, lpBaseAddress uint64, v byte) {
	WriteProcessMemory(h, lpBaseAddress, []byte{v}, 1)
}

func WriteProcessMemoryAsUint64(h HANDLE, lpBaseAddress uint64, v uint64) {
	data := binary.LittleEndian.AppendUint64(nil, v)
	WriteProcessMemory(h, lpBaseAddress, data, 8)
}

func WriteProcessMemoryAsFloat32(h HANDLE, lpBaseAddress uint64, v float32) {
	data := binary.LittleEndian.AppendUint32(nil, math.Float32bits(v))
	WriteProcessMemory(h, lpBaseAddress, data, 4)
}

func PostMessageAsRune(hwnd HWND, msg uint32, wParam rune) error {
	return PostMessage(hwnd, msg, uint32(wParam))
}
