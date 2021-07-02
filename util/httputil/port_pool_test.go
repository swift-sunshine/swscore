package httputil

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetFreePort(t *testing.T) {
	assert := assert.New(t)

	ResetPool()

	port := GetFreePort()
	assert.Equal(15000, port)

	port = GetFreePort()
	assert.Equal(15001, port)

	port = GetFreePort()
	assert.Equal(15002, port)
}

func TestGetFreePort_NoPortsAvailable(t *testing.T) {
	assert := assert.New(t)

	ResetPool()

	for i := 0; i < 100; i++ {
		port := GetFreePort()
		assert.Equal(15000+i, port)
	}

	port := GetFreePort()
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
	err := FreePort(15004)
	port = GetFreePort()
	assert.NoError(err)
	assert.Equal(15004, port)

	port = GetFreePort()
	assert.NoError(err)
	assert.Equal(0, port)

	err = FreePort(15004)
	port = GetFreePort()
	assert.NoError(err)
	assert.Equal(15004, port)

	err = FreePort(15099)
	port = GetFreePort()
	assert.NoError(err)
	assert.Equal(15099, port)

	err = FreePort(15000)
	port = GetFreePort()
	assert.NoError(err)
	assert.Equal(15000, port)
}

func TestFreePort_OutOfRange(t *testing.T) {
	err := FreePort(8080)
	assert.Errorf(t, err, "Port %d is out of range", 8080)
}

func TestResetPortPool(t *testing.T) {
	ResetPool()

	port := GetFreePort()
	assert.Equal(t, 15000, port)

	ResetPool()

	port = GetFreePort()
	assert.Equal(t, 15000, port)
}
