package media

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"syscall"
	"time"
)

const maxRemoteFileSize = 50 * 1024 * 1024 // 50MB

// cgnat is 100.64.0.0/10, which net.IP.IsPrivate does not cover.
var cgnat = &net.IPNet{IP: net.IPv4(100, 64, 0, 0), Mask: net.CIDRMask(10, 32)}

func isBlockedIP(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsMulticast() || ip.IsUnspecified() || cgnat.Contains(ip)
}

// blockInternalDial runs after DNS resolution on every connection attempt (including
// redirects), so hostnames resolving to internal addresses and DNS rebinding are rejected.
func blockInternalDial(network, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}
	ip := net.ParseIP(host)
	if ip == nil || isBlockedIP(ip) {
		return errors.New("destination address is not allowed")
	}
	return nil
}

// newRemoteFetchClient returns an HTTP client for user-supplied URLs (SSRF-hardened).
func newRemoteFetchClient() *http.Client {
	dialer := &net.Dialer{Timeout: 10 * time.Second, Control: blockInternalDial}
	return &http.Client{
		Timeout:   30 * time.Second,
		Transport: &http.Transport{DialContext: dialer.DialContext, Proxy: nil},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("too many redirects")
			}
			if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
				return fmt.Errorf("redirect to unsupported scheme %q", req.URL.Scheme)
			}
			return nil
		},
	}
}
