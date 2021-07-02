package httputil

import (
	"fmt"
	"sync"
)

// [1500] => true // busy
// [1501] => true // busy
// [1502] => false // free
// last port assigned => 1501
var portRangeInit = 15000
var portRangeSize = 100
var portsMap = map[int]bool{}
var lastFreePort = portRangeInit
var mutex sync.Mutex

// GetFreePort returns a non-busy port available within the reserved port range (15000 - 15099).
// The returned port is instantaneously marked as busy until is not freed using the FreePort method.
func GetFreePort() int {
	mutex.Lock()

	busy := portsMap[lastFreePort]
	freePortFound := 0
	attempts := 0
	for busy && attempts < portRangeSize {
		// If the pointer is getting out of range, restart from the beginning
		if lastFreePort >= portRangeInit+portRangeSize-1 {
			lastFreePort = portRangeInit
			// If the pointer is inside the range, increment by 1
		} else {
			lastFreePort++
		}

		busy = portsMap[lastFreePort]
		attempts++
	}

	if !busy {
		portsMap[lastFreePort] = true
		freePortFound = lastFreePort
	}

	mutex.Unlock()
	return freePortFound
}

// FreePort frees the port and makes it available for being pick to use.
func FreePort(port int) (err error) {
	if port < portRangeInit || port > portRangeInit+portRangeSize {
		return fmt.Errorf("port %d is out of range", port)
	}
	mutex.Lock()
	portsMap[port] = false
	mutex.Unlock()
	return err
}

func ResetPool() {
	mutex.Lock()

	lastFreePort = portRangeInit
	portsMap = map[int]bool{}

	mutex.Unlock()
}
