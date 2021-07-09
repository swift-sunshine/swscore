package httputil

import (
	"fmt"
	"sync"
)

type PortPool struct {
	// lastBusyPort is a pointer to the last free port given, therefore is in use.
	LastBusyPort int

	// mutex is the mutex used to solve concurrency problems while managing the port
	Mutex sync.Mutex

	// portsMap tracks whether an specific port is busy
	// portsMap[14100] = true => means that port 14100 is busy
	// portsMap[14101] = false => means that port 14101 is free
	PortsMap map[int]bool

	// portRangeInit is the first port number managed in the pool
	PortRangeInit int

	// portRangeSize is the size of the port range.
	// for example, the pool with portRangeSize 100 and portRangeInit 14000 manages
	// the ports from 14000 to 14099.
	PortRangeSize int
}

var Pool = &PortPool{
	LastBusyPort:  13999,
	Mutex:         sync.Mutex{},
	PortsMap:      map[int]bool{},
	PortRangeInit: 14000,
	PortRangeSize: 1000,
}

// GetFreePort returns a non-busy port available within the reserved port range (14100 - 14199).
// The returned port is instantaneously marked as busy until is not freed using the FreePort method.
func (pool *PortPool) GetFreePort() int {
	pool.Mutex.Lock()

	busy := true
	freePortFound := 0
	attempts := 0
	for busy && attempts < pool.PortRangeSize {
		// If the pointer is getting out of range, restart from the beginning
		if pool.LastBusyPort >= pool.PortRangeInit+pool.PortRangeSize-1 {
			pool.LastBusyPort = pool.PortRangeInit
			// If the pointer is inside the range, increment by 1
		} else {
			pool.LastBusyPort++
		}

		busy = pool.PortsMap[pool.LastBusyPort]
		attempts++
	}

	if !busy {
		pool.PortsMap[pool.LastBusyPort] = true
		freePortFound = pool.LastBusyPort
	}

	pool.Mutex.Unlock()
	return freePortFound
}

// FreePort frees the port and makes it available for being pick to use.
func (pool *PortPool) FreePort(port int) (err error) {
	if port < pool.PortRangeInit || port > pool.PortRangeInit+pool.PortRangeSize {
		return fmt.Errorf("port %d is out of range", port)
	}
	pool.Mutex.Lock()
	pool.PortsMap[port] = false
	pool.Mutex.Unlock()
	return err
}
