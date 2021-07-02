package httputil

import (
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"

	"github.com/kiali/kiali/log"
)

type PortForwarder interface {
	Start() error
	Stop()
}

type forwarder struct {
	forwarder *portforward.PortForwarder
	ReadyCh   chan struct{}
	StopCh    chan struct{}
	localPort int
}

func (f forwarder) Start() error {
	// It starts the port-forward
	errCh := make(chan error, 1)
	go func() {
		errCh <- f.forwarder.ForwardPorts()
	}()

	// Waiting until the ReadyChan has a value
	select {
	case err := <-errCh:
		log.Error("Failing starting the port forwarding")
		return err
	case <-f.ReadyCh:
		// Ready to forward requests
		return nil
	}
}

func (f forwarder) Stop() {
	// Closing the StopCh channel is closing the forwarding
	close(f.StopCh)
	err := FreePort(f.localPort)
	if err != nil {
		log.Errorf("Error stopping a port-forwarder: %v", err)
	}
}

func NewPortForwarder(client *rest.Interface, clientConfig *rest.Config, namespace, pod, address, portMap string, writer io.Writer) (*PortForwarder, error) {
	stopCh := make(chan struct{})
	readyCh := make(chan struct{})

	forwarderUrl := (*client).Post().
		Namespace(namespace).
		Resource("pods").
		Name(pod).
		SubResource("portforward").URL()

	transport, upgrader, err := spdy.RoundTripperFor(clientConfig)
	if err != nil {
		log.Errorf("Error creating a RoundTripper: %v", err)
		return nil, err
	}

	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, http.MethodPost, forwarderUrl)
	fwer, err := portforward.NewOnAddresses(dialer, []string{address}, []string{portMap},
		stopCh, readyCh, writer, os.Stderr)

	if err != nil {
		log.Errorf("Error creating the port-forwarder: %v", err)
		return nil, err
	}

	localPort, err := strconv.Atoi(strings.Split(portMap, ":")[0])
	if err != nil {
		log.Errorf("wrong port mapping between local port and destination port: %v", err)
		return nil, err
	}

	f := PortForwarder(forwarder{
		forwarder: fwer,
		ReadyCh:   readyCh,
		StopCh:    stopCh,
		localPort: localPort,
	})

	return &f, nil
}
