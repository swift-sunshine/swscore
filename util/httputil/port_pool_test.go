package httputil

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetFreePort(t *testing.T) {
	assert := assert.New(t)

	ResetPool()

	port := GetFreePort()
	assert.Equal(14100, port)

	port = GetFreePort()
	assert.Equal(14101, port)

	port = GetFreePort()
	assert.Equal(14102, port)
}

func TestGetFreePort_NoPortsAvailable(t *testing.T) {
	assert := assert.New(t)

	ResetPool()

	for i := 0; i < 100; i++ {
		port := GetFreePort()
		assert.Equal(14100+i, port)
	}

	port := GetFreePort()
	assert.Equal(0, port)

	port = GetFreePort()
	assert.Equal(0, port)
}

func TestFreePort(t *testing.T) {
	assert := assert.New(t)

	ResetPool()

	for i := 0; i < portRangeSize; i++ {
		port := GetFreePort()
		assert.Equal(portRangeInit+i, port)
	}

	// No free port available (out of range)
	port := GetFreePort()
	assert.Equal(0, port)

	// Once you free one port, this is
	err := FreePort(14104)
	port = GetFreePort()
	assert.NoError(err)
	assert.Equal(14104, port)

	port = GetFreePort()
	assert.NoError(err)
	assert.Equal(0, port)

	err = FreePort(14104)
	port = GetFreePort()
	assert.NoError(err)
	assert.Equal(14104, port)

	err = FreePort(14199)
	port = GetFreePort()
	assert.NoError(err)
	assert.Equal(14199, port)

	err = FreePort(14100)
	port = GetFreePort()
	assert.NoError(err)
	assert.Equal(14100, port)
}

func TestFreePort_OutOfRange(t *testing.T) {
	err := FreePort(8080)
	assert.Errorf(t, err, "Port %d is out of range", 8080)
}

func TestResetPortPool(t *testing.T) {
	ResetPool()

	port := GetFreePort()
	assert.Equal(t, 14100, port)

	ResetPool()

	port = GetFreePort()
	assert.Equal(t, 14100, port)
}
