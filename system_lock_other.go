//go:build !darwin && !windows

package main

func startSystemLockMonitor(func()) func() {
	return func() {}
}
