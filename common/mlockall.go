// +build mlockall,linux

package common

import "syscall"

func init() {
	println("Locking all memory")
	syscall.Mlockall(syscall.MCL_CURRENT | syscall.MCL_FUTURE)
}
