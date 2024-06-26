package spdy

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"

	"yunion.io/x/onecloud/pkg/util/httpstream"
	"yunion.io/x/onecloud/pkg/util/httpstream/spdy"
)

// Upgrader validates a response from the server after a SPDY upgrade.
type Upgrader interface {
	// NewConnection validates the response and creates a new Connection.
	NewConnection(resp *http.Response) (httpstream.Connection, error)
}

// RoundTripperFor returns a round tripper and upgrader to use with SPDY.
func RoundTripperFor() (http.RoundTripper, Upgrader, error) {
	// TODO: implement: k8s.io/kubernetes/staging/src/k8s.io/client-go/transport/spdy/spdy.go
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
	}
	upgradeRoundRipper := spdy.NewRoundTripper(tlsConfig, true, false)

	//return nil, upgradeRoundRipper, nil
	return upgradeRoundRipper, upgradeRoundRipper, nil
}

// dialer implements the httpstream.Dialer interface.
type dialer struct {
	client   *http.Client
	upgrader Upgrader
	method   string
	url      *url.URL
}

var _ httpstream.Dialer = &dialer{}

// NewDialer will create a dialer that connects to the provided URL and upgrades the connection to SPDY.
func NewDialer(upgrader Upgrader, client *http.Client, method string, url *url.URL) httpstream.Dialer {
	return &dialer{
		client:   client,
		upgrader: upgrader,
		method:   method,
		url:      url,
	}
}

func (d dialer) Dial(protocols ...string) (httpstream.Connection, string, error) {
	req, err := http.NewRequest(d.method, d.url.String(), nil)
	if err != nil {
		return nil, "", fmt.Errorf("error creating request: %v", err)
	}
	return Negotiate(d.upgrader, d.client, req, protocols...)
}

// Negotiate opens a connection to a remote server and attempts to negotiate
// a SPDY connection. Upon success, it returns the connection and the protocol selected by
// the server. The client transport must use the upgradeRoundTripper - see RoundTripperFor.
func Negotiate(upgrader Upgrader, client *http.Client, req *http.Request, protocols ...string) (httpstream.Connection, string, error) {
	for i := range protocols {
		req.Header.Add(httpstream.HeaderProtocolVersion, protocols[i])
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("error sending request: %v", err)
	}
	defer resp.Body.Close()
	conn, err := upgrader.NewConnection(resp)
	if err != nil {
		return nil, "", err
	}
	return conn, resp.Header.Get(httpstream.HeaderProtocolVersion), nil
}
