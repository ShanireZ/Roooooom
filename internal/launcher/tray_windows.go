//go:build windows

package launcher

import (
	"errors"
	"golang.org/x/sys/windows"
	"os"
	"runtime"
	"strconv"
	"unsafe"
)

var user32 = windows.NewLazySystemDLL("user32.dll")
var shell32 = windows.NewLazySystemDLL("shell32.dll")
var kernel32 = windows.NewLazySystemDLL("kernel32.dll")

type point struct{ X, Y int32 }
type winMessage struct {
	Hwnd           uintptr
	Msg            uint32
	Wparam, Lparam uintptr
	Time           uint32
	Pt             point
	Private        uint32
}
type windowClass struct {
	Size, Style                        uint32
	Proc                               uintptr
	ClassExtra, WindowExtra            int32
	Instance, Icon, Cursor, Background uintptr
	Menu, Name                         *uint16
	SmallIcon                          uintptr
}
type notifyIcon struct {
	Size                uint32
	Hwnd                uintptr
	ID, Flags, Callback uint32
	Icon                uintptr
	Tip                 [128]uint16
	State, StateMask    uint32
	Info                [256]uint16
	Version             uint32
	InfoTitle           [64]uint16
	InfoFlags           uint32
	Guid                [16]byte
	BalloonIcon         uintptr
}

func trayClassName() string  { return "RoooooomTray." + strconv.Itoa(os.Getpid()) }
func utf16(s string) *uint16 { p, _ := windows.UTF16PtrFromString(s); return p }
func (Native) ShowTray(menu Menu) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	instance, _, _ := kernel32.NewProc("GetModuleHandleW").Call(0)
	icon, _, _ := user32.NewProc("LoadIconW").Call(0, 32512)
	alerts := make(chan bool, 1)
	post := func(hwnd uintptr, msg, w, l uintptr) { user32.NewProc("PostMessageW").Call(hwnd, msg, w, l) }
	callback := windows.NewCallback(func(hwnd uintptr, msg uint32, wparam, lparam uintptr) uintptr {
		switch msg {
		case 0x8001:
			switch lparam & 0xffff {
			case 0x0202, 0x0400, 0x0401:
				go func() {
					if menu.Open() != nil {
						select {
						case alerts <- true:
						default:
						}
						post(hwnd, 0x8002, 0, 0)
					}
				}()
			case 0x0205, 0x007b:
				var pos point
				user32.NewProc("GetCursorPos").Call(uintptr(unsafe.Pointer(&pos)))
				popup, _, _ := user32.NewProc("CreatePopupMenu").Call()
				if popup != 0 {
					defer user32.NewProc("DestroyMenu").Call(popup)
					for _, item := range []struct {
						id    uintptr
						label string
					}{{1, "開啟看板"}, {4, "重新啟用"}, {2, "退出托盤（服務繼續）"}, {3, "停止服務並退出"}} {
						if item.id == 4 && menu.Activate == nil {
							continue
						}
						user32.NewProc("AppendMenuW").Call(popup, 0, item.id, uintptr(unsafe.Pointer(utf16(item.label))))
					}
					user32.NewProc("SetForegroundWindow").Call(hwnd)
					user32.NewProc("TrackPopupMenu").Call(popup, 0, uintptr(pos.X), uintptr(pos.Y), 0, hwnd, 0)
					post(hwnd, 0, 0, 0)
				}
			}
			return 0
		case 0x0111:
			switch wparam & 0xffff {
			case 1:
				go func() {
					if menu.Open() != nil {
						select {
						case alerts <- true:
						default:
						}
						post(hwnd, 0x8002, 0, 0)
					}
				}()
			case 2:
				post(hwnd, 0x0010, 0, 0)
			case 4:
				if menu.Activate != nil {
					go func() {
						if menu.Activate() != nil {
							select {
							case alerts <- true:
							default:
							}
							post(hwnd, 0x8002, 0, 0)
						}
					}()
				}
			case 3:
				go func() {
					if menu.Stop() == nil {
						post(hwnd, 0x0010, 0, 0)
					} else {
						select {
						case alerts <- true:
						default:
						}
						post(hwnd, 0x8002, 0, 0)
					}
				}()
			}
			return 0
		case 0x8002:
			select {
			case <-alerts:
				user32.NewProc("MessageBoxW").Call(hwnd, uintptr(unsafe.Pointer(utf16("本機操作未確認。請重新開啟啟動器，核對本產品的服務與資料目錄。"))), uintptr(unsafe.Pointer(utf16("Roooooom"))), 0x10)
			default:
			}
			return 0
		case 0x0010:
			user32.NewProc("DestroyWindow").Call(hwnd)
			return 0
		case 0x0002:
			user32.NewProc("PostQuitMessage").Call(0)
			return 0
		}
		result, _, _ := user32.NewProc("DefWindowProcW").Call(hwnd, uintptr(msg), wparam, lparam)
		return result
	})
	name := utf16(trayClassName())
	class := windowClass{Proc: callback, Instance: instance, Icon: icon, Name: name}
	class.Size = uint32(unsafe.Sizeof(class))
	atom, _, _ := user32.NewProc("RegisterClassExW").Call(uintptr(unsafe.Pointer(&class)))
	if atom == 0 {
		return ErrTrayUnavailable
	}
	defer user32.NewProc("UnregisterClassW").Call(uintptr(unsafe.Pointer(name)), instance)
	hwnd, _, _ := user32.NewProc("CreateWindowExW").Call(0, uintptr(unsafe.Pointer(name)), uintptr(unsafe.Pointer(utf16("Roooooom"))), 0x80000000, 0, 0, 0, 0, 0, 0, instance, 0)
	if hwnd == 0 {
		return ErrTrayUnavailable
	}
	defer user32.NewProc("DestroyWindow").Call(hwnd)
	data := notifyIcon{Hwnd: hwnd, ID: 1, Flags: 0x1 | 0x2 | 0x4, Callback: 0x8001, Icon: icon}
	data.Size = uint32(unsafe.Sizeof(data))
	copy(data.Tip[:], windows.StringToUTF16("Roooooom · 本機協作服務"))
	ok, _, _ := shell32.NewProc("Shell_NotifyIconW").Call(0, uintptr(unsafe.Pointer(&data)))
	if ok == 0 {
		return ErrTrayUnavailable
	}
	defer shell32.NewProc("Shell_NotifyIconW").Call(2, uintptr(unsafe.Pointer(&data)))
	data.Version = 4
	shell32.NewProc("Shell_NotifyIconW").Call(4, uintptr(unsafe.Pointer(&data)))
	var message winMessage
	for {
		result, _, _ := user32.NewProc("GetMessageW").Call(uintptr(unsafe.Pointer(&message)), 0, 0, 0)
		if int32(result) == -1 {
			return errors.New("own tray message loop failed")
		}
		if result == 0 {
			return nil
		}
		user32.NewProc("TranslateMessage").Call(uintptr(unsafe.Pointer(&message)))
		user32.NewProc("DispatchMessageW").Call(uintptr(unsafe.Pointer(&message)))
	}
}
