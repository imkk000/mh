//go:build windows

package mh

import (
	"fmt"
	"syscall"
)

func GetBaseAddress(pid uint32) (uint64, error) {
	h, err := CreateToolhelp32Snapshot(TH32CS_SNAPMODULE|TH32CS_SNAPMODULE32, pid)
	if err != nil {
		return 0, fmt.Errorf("snapshot: %w", err)
	}
	defer CloseHandle(h)
	var m MODULEENTRY32
	if !Module32First(h, &m) {
		return 0, fmt.Errorf("no modules in pid %d", pid)
	}
	return uint64(m.ModBaseAddr), nil
}

func FindModuleBase(pid uint32, name string) (uint64, error) {
	h, err := CreateToolhelp32Snapshot(TH32CS_SNAPMODULE|TH32CS_SNAPMODULE32, pid)
	if err != nil {
		return 0, fmt.Errorf("snapshot: %w", err)
	}
	defer CloseHandle(h)
	var m MODULEENTRY32
	ok := Module32First(h, &m)
	for ok {
		if syscall.UTF16ToString(m.SzModule[:]) == name {
			return uint64(m.ModBaseAddr), nil
		}
		ok = Module32Next(h, &m)
	}
	return 0, fmt.Errorf("module %q not found in pid %d", name, pid)
}

func GetPID(targetName string) (uint32, error) {
	h, err := CreateToolhelp32Snapshot(TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return 0, fmt.Errorf("snapshot: %w", err)
	}
	defer CloseHandle(h)
	var pe PROCESSENTRY32
	ok := Process32First(h, &pe)
	for ok {
		if syscall.UTF16ToString(pe.ExeFile[:]) == targetName {
			return pe.ProcessID, nil
		}
		ok = Process32Next(h, &pe)
	}
	return 0, fmt.Errorf("process %q not found", targetName)
}
