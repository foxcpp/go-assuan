// +build mlockall,linux

package common

import "syscall"

func init() {
	Logger.Println("Locking all memory...")
	if err := syscall.Mlockall(syscall.MCL_CURRENT | syscall.MCL_FUTURE); err != nil {
		Logger.Println("... failed:" err)
	}
}
