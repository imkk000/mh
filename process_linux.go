//go:build linux

package mh

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/jezek/xgb"
	"github.com/jezek/xgb/xproto"
	"github.com/jezek/xgb/xtest"
	"golang.org/x/sys/unix"
)

type linuxProcess struct {
	pid       int
	baseAddr  uint64
	exeSuffix string

	x      *xConn
	xerr   error
	xMu    sync.Mutex
	wid    xproto.Window
	wmName string
}

type xConn struct {
	c        *xgb.Conn
	root     xproto.Window
	keysyms  map[xproto.Keysym]xproto.Keycode
	minCode  xproto.Keycode
	maxCode  xproto.Keycode
	activeAt xproto.Atom
	pidAt    xproto.Atom
	clientAt xproto.Atom
}

func Attach(processName string) (Process, error) {
	return AttachWithClass(processName, "Path of Exile 2")
}

func isRunning(processName string) bool {
	_, err := findPID(processName)
	return err == nil
}

func AttachWithClass(processName, wmName string) (Process, error) {
	pid, err := findPID(processName)
	if err != nil {
		return nil, err
	}
	base, err := findModuleBaseLinux(pid, processName)
	if err != nil {
		return nil, fmt.Errorf("module base: %w", err)
	}
	return &linuxProcess{
		pid:       pid,
		baseAddr:  base,
		exeSuffix: processName,
		wmName:    wmName,
	}, nil
}

func findPID(exeSuffix string) (int, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return 0, err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		raw, err := os.ReadFile("/proc/" + e.Name() + "/cmdline")
		if err != nil {
			continue
		}
		nul := strings.IndexByte(string(raw), 0)
		if nul < 0 {
			nul = len(raw)
		}
		argv0 := string(raw[:nul])
		if strings.HasSuffix(argv0, exeSuffix) {
			return pid, nil
		}
	}
	return 0, fmt.Errorf("process %q not found", exeSuffix)
}

func findModuleBaseLinux(pid int, exeSuffix string) (uint64, error) {
	maps, err := os.ReadFile(fmt.Sprintf("/proc/%d/maps", pid))
	if err != nil {
		return 0, err
	}
	for _, line := range strings.Split(string(maps), "\n") {
		if !strings.HasSuffix(line, exeSuffix) {
			continue
		}
		dash := strings.IndexByte(line, '-')
		if dash <= 0 {
			continue
		}
		base, err := strconv.ParseUint(line[:dash], 16, 64)
		if err != nil {
			continue
		}
		return base, nil
	}
	return 0, fmt.Errorf("no mapping for %q in /proc/%d/maps", exeSuffix, pid)
}

func (p *linuxProcess) PID() uint32        { return uint32(p.pid) }
func (p *linuxProcess) ModuleBase() uint64 { return p.baseAddr }

func (p *linuxProcess) Close() error {
	p.xMu.Lock()
	defer p.xMu.Unlock()
	if p.x != nil {
		p.x.c.Close()
		p.x = nil
	}
	return nil
}

func (p *linuxProcess) Read(addr uint64, n int) ([]byte, error) {
	if n <= 0 {
		return nil, nil
	}
	buf := make([]byte, n)
	local := []unix.Iovec{{Base: &buf[0], Len: uint64(n)}}
	remote := []unix.RemoteIovec{{Base: uintptr(addr), Len: n}}
	got, err := unix.ProcessVMReadv(p.pid, local, remote, 0)
	if err != nil {
		return nil, err
	}
	if got != n {
		return buf[:got], fmt.Errorf("%w at %X: %d of %d", ErrShortRead, addr, got, n)
	}
	return buf, nil
}

func (p *linuxProcess) ReadU8(addr uint64) (byte, error) {
	data, err := p.Read(addr, 1)
	if err != nil || len(data) < 1 {
		return 0, err
	}
	return data[0], nil
}

func (p *linuxProcess) ReadU32(addr uint64) (uint32, error) {
	data, err := p.Read(addr, 4)
	if err != nil || len(data) < 4 {
		return 0, err
	}
	return binary.LittleEndian.Uint32(data), nil
}

func (p *linuxProcess) ReadU64(addr uint64) (uint64, error) {
	data, err := p.Read(addr, 8)
	if err != nil || len(data) < 8 {
		return 0, err
	}
	return binary.LittleEndian.Uint64(data), nil
}

func (p *linuxProcess) ReadFloat32(addr uint64) (float32, error) {
	data, err := p.Read(addr, 4)
	if err != nil || len(data) < 4 {
		return 0, err
	}
	return math.Float32frombits(binary.LittleEndian.Uint32(data)), nil
}

func (p *linuxProcess) ReadOffsets(base uint64, offsets ...uint64) uint64 {
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

func (p *linuxProcess) WriteU8(addr uint64, v byte) error {
	return p.writeBytes(addr, []byte{v})
}

func (p *linuxProcess) WriteFloat32(addr uint64, v float32) error {
	data := binary.LittleEndian.AppendUint32(nil, math.Float32bits(v))
	return p.writeBytes(addr, data)
}

func (p *linuxProcess) writeBytes(addr uint64, data []byte) error {
	if len(data) == 0 {
		return nil
	}
	local := []unix.Iovec{{Base: &data[0], Len: uint64(len(data))}}
	remote := []unix.RemoteIovec{{Base: uintptr(addr), Len: len(data)}}
	got, err := unix.ProcessVMWritev(p.pid, local, remote, 0)
	if err != nil {
		return err
	}
	if got != len(data) {
		return fmt.Errorf("mh: short write at %X: %d of %d", addr, got, len(data))
	}
	return nil
}

func (p *linuxProcess) ReadableRegions() ([]Region, error) {
	maps, err := os.ReadFile(fmt.Sprintf("/proc/%d/maps", p.pid))
	if err != nil {
		return nil, err
	}
	var regions []Region
	for _, line := range strings.Split(string(maps), "\n") {
		if line == "" {
			continue
		}
		dash := strings.IndexByte(line, '-')
		if dash <= 0 {
			continue
		}
		space := strings.IndexByte(line, ' ')
		if space <= dash {
			continue
		}
		start, err := strconv.ParseUint(line[:dash], 16, 64)
		if err != nil {
			continue
		}
		end, err := strconv.ParseUint(line[dash+1:space], 16, 64)
		if err != nil {
			continue
		}
		if space+5 > len(line) {
			continue
		}
		perms := line[space+1 : space+5]
		if perms[0] != 'r' {
			continue
		}
		regions = append(regions, Region{
			Start:      start,
			End:        end,
			Executable: perms[2] == 'x',
			Writable:   perms[1] == 'w',
		})
	}
	return regions, nil
}

func (p *linuxProcess) ensureX() (*xConn, error) {
	p.xMu.Lock()
	defer p.xMu.Unlock()
	if p.x != nil {
		return p.x, nil
	}
	if p.xerr != nil {
		return nil, p.xerr
	}
	x, err := openXConn()
	if err != nil {
		p.xerr = err
		return nil, err
	}
	p.x = x
	return x, nil
}

func openXConn() (*xConn, error) {
	c, err := xgb.NewConn()
	if err != nil {
		return nil, err
	}
	if err := xtest.Init(c); err != nil {
		c.Close()
		return nil, fmt.Errorf("xtest init: %w", err)
	}
	setup := xproto.Setup(c)
	screen := setup.DefaultScreen(c)
	mapping, err := xproto.GetKeyboardMapping(c, setup.MinKeycode, byte(setup.MaxKeycode-setup.MinKeycode+1)).Reply()
	if err != nil {
		c.Close()
		return nil, fmt.Errorf("keyboard mapping: %w", err)
	}
	syms := make(map[xproto.Keysym]xproto.Keycode)
	per := int(mapping.KeysymsPerKeycode)
	for i := 0; i < int(setup.MaxKeycode-setup.MinKeycode+1); i++ {
		code := xproto.Keycode(int(setup.MinKeycode) + i)
		for j := 0; j < per; j++ {
			s := mapping.Keysyms[i*per+j]
			if s == 0 {
				continue
			}
			if _, exists := syms[s]; !exists {
				syms[s] = code
			}
		}
	}
	activeAt, err := internAtom(c, "_NET_ACTIVE_WINDOW")
	if err != nil {
		c.Close()
		return nil, err
	}
	pidAt, err := internAtom(c, "_NET_WM_PID")
	if err != nil {
		c.Close()
		return nil, err
	}
	clientAt, err := internAtom(c, "_NET_CLIENT_LIST")
	if err != nil {
		c.Close()
		return nil, err
	}
	return &xConn{
		c:        c,
		root:     screen.Root,
		keysyms:  syms,
		minCode:  setup.MinKeycode,
		maxCode:  setup.MaxKeycode,
		activeAt: activeAt,
		pidAt:    pidAt,
		clientAt: clientAt,
	}, nil
}

func internAtom(c *xgb.Conn, name string) (xproto.Atom, error) {
	r, err := xproto.InternAtom(c, true, uint16(len(name)), name).Reply()
	if err != nil {
		return 0, err
	}
	return r.Atom, nil
}

func (p *linuxProcess) gameWindow() (xproto.Window, bool) {
	x, err := p.ensureX()
	if err != nil {
		return 0, false
	}
	if p.wid != 0 {
		return p.wid, true
	}
	prop, err := xproto.GetProperty(x.c, false, x.root, x.clientAt, xproto.AtomWindow, 0, 1024).Reply()
	if err != nil || prop == nil {
		return 0, false
	}
	nameAt := xproto.Atom(xproto.AtomWmName)
	n := len(prop.Value) / 4
	for i := 0; i < n; i++ {
		w := xproto.Window(binary.LittleEndian.Uint32(prop.Value[i*4:]))
		name, _ := xproto.GetProperty(x.c, false, w, nameAt, xproto.AtomString, 0, 256).Reply()
		if name == nil {
			continue
		}
		if strings.TrimSpace(string(name.Value)) != p.wmName {
			continue
		}
		p.wid = w
		return w, true
	}
	return 0, false
}

func (p *linuxProcess) IsForeground() bool {
	x, err := p.ensureX()
	if err != nil {
		return true
	}
	prop, err := xproto.GetProperty(x.c, false, x.root, x.activeAt, xproto.AtomWindow, 0, 1).Reply()
	if err != nil || prop == nil || len(prop.Value) < 4 {
		return false
	}
	active := xproto.Window(binary.LittleEndian.Uint32(prop.Value))
	wid, ok := p.gameWindow()
	if !ok {
		return false
	}
	return active == wid
}

func (p *linuxProcess) SendMouseClick(btn MouseButton) error {
	x, err := p.ensureX()
	if err != nil {
		return err
	}
	var x11btn byte
	switch btn {
	case MouseLeft:
		x11btn = 1
	case MouseMiddle:
		x11btn = 2
	case MouseRight:
		x11btn = 3
	default:
		return fmt.Errorf("mh: unknown mouse button %d", btn)
	}
	if err := xtest.FakeInputChecked(x.c, xproto.ButtonPress, x11btn, 0, x.root, 0, 0, 0).Check(); err != nil {
		return err
	}
	return xtest.FakeInputChecked(x.c, xproto.ButtonRelease, x11btn, 0, x.root, 0, 0, 0).Check()
}

func (p *linuxProcess) SendChar(r rune) error {
	x, err := p.ensureX()
	if err != nil {
		return err
	}
	code, ok := x.keysyms[xproto.Keysym(runeToKeysym(r))]
	if !ok {
		return fmt.Errorf("mh: no keycode for rune %q", r)
	}
	if err := xtest.FakeInputChecked(x.c, xproto.KeyPress, byte(code), 0, x.root, 0, 0, 0).Check(); err != nil {
		return err
	}
	return xtest.FakeInputChecked(x.c, xproto.KeyRelease, byte(code), 0, x.root, 0, 0, 0).Check()
}

func runeToKeysym(r rune) uint32 {
	if r >= 0x20 && r <= 0x7e {
		return uint32(r)
	}
	return uint32(r)
}

func (p *linuxProcess) IsKeyDown(k Key) bool {
	x, err := p.ensureX()
	if err != nil {
		return false
	}
	switch k {
	case KeyLeftMouse, KeyRightMouse, KeyMiddleMouse, KeyXButton1, KeyXButton2:
		ptr, err := xproto.QueryPointer(x.c, x.root).Reply()
		if err != nil {
			return false
		}
		mask := uint16(ptr.Mask)
		const (
			button1Mask = 1 << 8
			button2Mask = 1 << 9
			button3Mask = 1 << 10
			button4Mask = 1 << 11
			button5Mask = 1 << 12
		)
		switch k {
		case KeyLeftMouse:
			return mask&button1Mask != 0
		case KeyMiddleMouse:
			return mask&button2Mask != 0
		case KeyRightMouse:
			return mask&button3Mask != 0
		case KeyXButton1:
			return mask&button4Mask != 0
		case KeyXButton2:
			return mask&button5Mask != 0
		}
	case KeySpace, KeyF2, KeyF3, KeyF4:
		sym := keysymForKey(k)
		if sym == 0 {
			return false
		}
		code, ok := x.keysyms[xproto.Keysym(sym)]
		if !ok {
			return false
		}
		reply, err := xproto.QueryKeymap(x.c).Reply()
		if err != nil {
			return false
		}
		idx := int(code) / 8
		bit := byte(1 << (uint(code) % 8))
		if idx >= len(reply.Keys) {
			return false
		}
		return reply.Keys[idx]&bit != 0
	}
	return false
}

func keysymForKey(k Key) uint32 {
	switch k {
	case KeySpace:
		return 0x0020
	case KeyF2:
		return 0xFFBF
	case KeyF3:
		return 0xFFC0
	case KeyF4:
		return 0xFFC1
	}
	return 0
}
