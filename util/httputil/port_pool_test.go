package httputil

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetFreePort(t *testing.T) {
	assert := assert.New(t)

	testPool := newPool(14100, 100)

	port := testPool.GetFreePort()
	assert.Equal(14100, port)

	port = testPool.GetFreePort()
	assert.Equal(14101, port)

	port = testPool.GetFreePort()
	assert.Equal(14102, port)
}

func TestGetFreePort_NoPortsAvailable(t *testing.T) {
	assert := assert.New(t)

	testPool := newPool(14100, 100)

	for i := 0; i < 100; i++ {
		port := testPool.GetFreePort()
		assert.Equal(14100+i, port)
	}

	port := testPool.GetFreePort()
	assert.Equal(0, port)

	port = testPool.GetFreePort()
	assert.Equal(0, port)
}

func TestFreePort(t *testing.T) {
	assert := assert.New(t)

	testPool := newPool(14100, 100)

	for i := 0; i < testPool.PortRangeSize; i++ {
		port := testPool.GetFreePort()
		assert.Equal(testPool.PortRangeInit+i, port)
	}

	// No free port available (out of range)
	port := testPool.GetFreePort()
	assert.Equal(0, port)

	// Once you free one port, this is
	err := testPool.FreePort(14104)
	port = testPool.GetFreePort()
	assert.NoError(err)
	assert.Equal(14104, port)

	port = testPool.GetFreePort()
	assert.NoError(err)
	assert.Equal(0, port)

	err = testPool.FreePort(14104)
	port = testPool.GetFreePort()
	assert.NoError(err)
	assert.Equal(14104, port)

	err = testPool.FreePort(14199)
	port = testPool.GetFreePort()
	assert.NoError(err)
	assert.Equal(14199, port)

	err = testPool.FreePort(14100)
	port = testPool.GetFreePort()
	assert.NoError(err)
	assert.Equal(14100, port)
}

func TestFreePort_OutOfRange(t *testing.T) {
	testPool := newPool(14100, 100)

	err := testPool.FreePort(8080)
	assert.Errorf(t, err, "Port %d is out of range", 8080)
}

func newPool(rangeInit, rangeSize int) PortPool {
	return PortPool{
		LastBusyPort:  rangeInit - 1,
		Mutex:         sync.Mutex{},
		PortsMap:      map[int]bool{},
		PortRangeInit: rangeInit,
		PortRangeSize: rangeSize,
	}
}
