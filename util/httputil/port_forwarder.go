package httputil

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"

	"github.com/kiali/kiali/kubernetes"
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
		log.Errorf("Failing starting the port forwarding")
		return err
	case <-f.ReadyCh:
		// Ready to forward requests
		return nil
	}
}

func (f forwarder) Stop() {
	// Closing the StopCh channel is closing the forwarding
	close(f.StopCh)
}

func NewPortForwarder(client rest.Interface, clientConfig *rest.Config, namespace, pod, address, portMap string, writer io.Writer) (*PortForwarder, error) {
	stopCh := make(chan struct{})
	readyCh := make(chan struct{})

	forwarderUrl := client.Post().
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

	f := PortForwarder(forwarder{
		forwarder: fwer,
		ReadyCh:   readyCh,
		StopCh:    stopCh,
	})

	return &f, nil
}

func ForwardGetRequest(client kubernetes.ClientInterface, namespace, podName string, localPort, destinationPort int, path string) ([]byte, error) {
	f, err := client.GetPodPortForwarder(namespace, podName, fmt.Sprintf("%d:%d", localPort, destinationPort))
	if err != nil {
		return nil, err
	}

	// Start the forwarding
	if err := (*f).Start(); err != nil {
		return nil, err
	}

	// Defering the finish of the port-forwarding
	defer (*f).Stop()

	// Ready to create a request
	resp, code, err := HttpGet(fmt.Sprintf("http://localhost:%d%s", localPort, path), nil, 10*time.Second)
	if code >= 400 {
		return resp, fmt.Errorf("error fetching the /config_dump for the Envoy. Response code: %d", code)
	}

	return resp, err
}
