package httputil

import (
	"fmt"
	"sync"
)

// lastBusyPort is a pointer to the last free port given, therefore is in use.
var lastBusyPort = portRangeInit - 1

// mutex is the mutex used to solve concurrency problems while managing the port
var mutex sync.Mutex

// portsMap tracks whether an specific port is busy
// portsMap[14100] = true => means that port 14100 is busy
// portsMap[14101] = false => means that port 14101 is free
var portsMap = map[int]bool{}

// portRangeInit is the first port number managed in the pool
var portRangeInit = 14100

// portRangeSize is the size of the port range.
// for example, the pool with portRangeSize 100 and portRangeInit 14000 manages
// the ports from 14000 to 14099.
var portRangeSize = 100

// GetFreePort returns a non-busy port available within the reserved port range (14100 - 14199).
// The returned port is instantaneously marked as busy until is not freed using the FreePort method.
func GetFreePort() int {
	mutex.Lock()

	busy := true
	freePortFound := 0
	attempts := 0
	for busy && attempts < portRangeSize {
		// If the pointer is getting out of range, restart from the beginning
		if lastBusyPort >= portRangeInit+portRangeSize-1 {
			lastBusyPort = portRangeInit
			// If the pointer is inside the range, increment by 1
		} else {
			lastBusyPort++
		}

		busy = portsMap[lastBusyPort]
		attempts++
	}

	if !busy {
		portsMap[lastBusyPort] = true
		freePortFound = lastBusyPort
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

	lastBusyPort = portRangeInit - 1
	portsMap = map[int]bool{}

	mutex.Unlock()
}
