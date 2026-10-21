//go:build darwin

package main

/*
void keynest_start_system_lock_monitor(void);
void keynest_stop_system_lock_monitor(void);
int keynest_take_system_lock_event(void);
*/
import "C"

import (
	"sync"
	"time"
)

const (
	darwinSystemLockPollPeriod = 50 * time.Millisecond
	darwinSleepGap             = 2 * time.Second
)

func startSystemLockMonitor(onLock func()) func() {
	C.keynest_start_system_lock_monitor()
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(darwinSystemLockPollPeriod)
		defer ticker.Stop()
		lastPoll := time.Now()
		for {
			select {
			case <-ticker.C:
				now := time.Now()
				resumedFromSleep := now.Sub(lastPoll) > darwinSleepGap
				lastPoll = now
				if C.keynest_take_system_lock_event() != 0 || resumedFromSleep {
					onLock()
				}
			case <-stop:
				return
			}
		}
	}()

	var once sync.Once
	return func() {
		once.Do(func() {
			C.keynest_stop_system_lock_monitor()
			close(stop)
			<-done
		})
	}
}
