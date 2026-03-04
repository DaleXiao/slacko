package transport

import (
	"context"
	"fmt"
	"net"
	"net/http"

	tls "github.com/refraction-networking/utls"
)

// NewEdgeTransport returns an http.Transport that mimics Microsoft Edge's
// TLS ClientHello fingerprint. This prevents Enterprise Grid security
// systems from detecting non-browser TLS connections (JA3/JA4 mismatch).
func NewEdgeTransport() *http.Transport {
	return &http.Transport{
		DialTLSContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			// Separate host:port
			host, _, err := net.SplitHostPort(addr)
			if err != nil {
				host = addr
			}

			// Dial TCP
			dialer := &net.Dialer{}
			conn, err := dialer.DialContext(ctx, network, addr)
			if err != nil {
				return nil, fmt.Errorf("tcp dial: %w", err)
			}

			// Wrap with uTLS using Chrome fingerprint (Edge = Chromium-based, same TLS)
			tlsConn := tls.UClient(conn, &tls.Config{
				ServerName: host,
			}, tls.HelloChrome_Auto)

			if err := tlsConn.HandshakeContext(ctx); err != nil {
				conn.Close()
				return nil, fmt.Errorf("tls handshake: %w", err)
			}

			return tlsConn, nil
		},
		ForceAttemptHTTP2: true,
	}
}
