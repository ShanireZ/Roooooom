//go:build windows

package launcher

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"
)

func TestWindowsLauncherNativeTrayMenu(t *testing.T) {
	for _, stop := range []bool{false, true} {
		t.Run(map[bool]string{false: "tray-exit", true: "service-stop"}[stop], func(t *testing.T) {
			var stops atomic.Int32
			opened := make(chan struct{}, 1)
			done := make(chan error, 1)
			go func() {
				done <- (Native{}).ShowTray(Menu{Open: func() error { opened <- struct{}{}; return nil }, Stop: func() error { stops.Add(1); return nil }})
			}()
			deadline := time.Now().Add(3 * time.Second)
			var hwnd uintptr
			for time.Now().Before(deadline) {
				select {
				case e := <-done:
					if errors.Is(e, ErrTrayUnavailable) {
						t.Skip("native notification area unavailable; no GUI support claim")
					}
					t.Fatal(e)
				default:
				}
				hwnd, _, _ = user32.NewProc("FindWindowW").Call(uintptr(unsafe.Pointer(utf16(trayClassName()))), 0)
				if hwnd != 0 {
					break
				}
				time.Sleep(10 * time.Millisecond)
			}
			if hwnd == 0 {
				t.Fatal("own tray window did not initialize")
			}
			t.Cleanup(func() { user32.NewProc("PostMessageW").Call(hwnd, 0x0010, 0, 0) })
			user32.NewProc("PostMessageW").Call(hwnd, 0x0111, 1, 0)
			select {
			case <-opened:
			case e := <-done:
				if errors.Is(e, ErrTrayUnavailable) {
					t.Skip("native tray unavailable")
				}
				t.Fatal(e)
			case <-time.After(3 * time.Second):
				t.Fatal("own open action not called")
			}
			id := uintptr(2)
			if stop {
				id = 3
			}
			user32.NewProc("PostMessageW").Call(hwnd, 0x0111, id, 0)
			select {
			case e := <-done:
				if e != nil {
					t.Fatal(e)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("own native tray did not exit")
			}
			want := int32(0)
			if stop {
				want = 1
			}
			if stops.Load() != want {
				t.Fatal("tray exit/stop conflated", stops.Load())
			}
		})
	}
}

func TestWindowsGuidedActivationMenu(t *testing.T) {
	activated := make(chan struct{}, 1)
	done := make(chan error, 1)
	go func() {
		done <- (Native{}).ShowTray(Menu{Open: func() error { return nil }, Stop: func() error { return nil }, Activate: func() error { activated <- struct{}{}; return nil }})
	}()
	deadline := time.Now().Add(3 * time.Second)
	var hwnd uintptr
	for time.Now().Before(deadline) {
		select {
		case e := <-done:
			if errors.Is(e, ErrTrayUnavailable) {
				t.Skip("native tray unavailable; GUI not verified")
			}
			t.Fatal(e)
		default:
		}
		hwnd, _, _ = user32.NewProc("FindWindowW").Call(uintptr(unsafe.Pointer(utf16(trayClassName()))), 0)
		if hwnd != 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if hwnd == 0 {
		t.Fatal("own tray window unavailable")
	}
	t.Cleanup(func() {
		user32.NewProc("PostMessageW").Call(hwnd, 0x0010, 0, 0)
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Error("tray cleanup timed out")
		}
	})
	user32.NewProc("PostMessageW").Call(hwnd, 0x0111, 4, 0)
	select {
	case <-activated:
	case <-time.After(1500 * time.Millisecond):
		t.Fatal("reactivation action was not called")
	}
}
