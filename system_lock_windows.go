//go:build windows

package main

import (
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	wtsCurrentSession    = 0xffffffff
	wtsInfoClassEx       = 25
	wtsInfoExLevel       = 1
	wtsSessionStateLock  = 0
	systemLockPollPeriod = time.Second
)

var (
	wtsapi32                   = windows.NewLazySystemDLL("wtsapi32.dll")
	wtsQuerySessionInformation = wtsapi32.NewProc("WTSQuerySessionInformationW")
	wtsFreeMemory              = wtsapi32.NewProc("WTSFreeMemory")
)

type wtsInfoEx struct {
	Level uint32
	Data  wtsInfoExLevel1
}

type wtsInfoExLevel1 struct {
	SessionID               uint32
	SessionState            uint32
	SessionFlags            int32
	WinStationName          [33]uint16
	UserName                [21]uint16
	DomainName              [18]uint16
	LogonTime               int64
	ConnectTime             int64
	DisconnectTime          int64
	LastInputTime           int64
	CurrentTime             int64
	IncomingBytes           uint32
	OutgoingBytes           uint32
	IncomingFrames          uint32
	OutgoingFrames          uint32
	IncomingCompressedBytes uint32
	OutgoingCompressedBytes uint32
}

func startSystemLockMonitor(onLock func()) func() {
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(systemLockPollPeriod)
		defer ticker.Stop()
		wasLocked := false
		for {
			select {
			case <-ticker.C:
				locked, ok := windowsSessionLocked()
				if ok && locked && !wasLocked {
					onLock()
				}
				if ok {
					wasLocked = locked
				}
			case <-stop:
				return
			}
		}
	}()

	var once sync.Once
	return func() {
		once.Do(func() {
			close(stop)
			<-done
		})
	}
}

func windowsSessionLocked() (bool, bool) {
	var buffer uintptr
	var bytesReturned uint32
	result, _, _ := wtsQuerySessionInformation.Call(
		0,
		uintptr(wtsCurrentSession),
		uintptr(wtsInfoClassEx),
		uintptr(unsafe.Pointer(&buffer)),
		uintptr(unsafe.Pointer(&bytesReturned)),
	)
	if result == 0 || buffer == 0 || bytesReturned < uint32(unsafe.Sizeof(wtsInfoEx{})) {
		return false, false
	}
	defer wtsFreeMemory.Call(buffer)

	info := (*wtsInfoEx)(unsafe.Pointer(buffer))
	if info.Level != wtsInfoExLevel {
		return false, false
	}
	return info.Data.SessionFlags == wtsSessionStateLock, true
}
