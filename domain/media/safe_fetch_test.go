package media

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
)

func TestIsBlockedIP(t *testing.T) {
	for ip, want := range map[string]bool{
		"127.0.0.1": true, "10.0.0.5": true, "192.168.1.1": true, "172.16.0.1": true,
		"169.254.169.254": true, "100.64.0.1": true, "0.0.0.0": true, "::1": true, "fe80::1": true, "fd00::1": true,
		"8.8.8.8": false, "93.184.216.34": false,
	} {
		if got := isBlockedIP(net.ParseIP(ip)); got != want {
			t.Errorf("isBlockedIP(%s) = %v, want %v", ip, got, want)
		}
	}
}

func TestUploadFromURLBlocksInternalTargets(t *testing.T) {
	hit := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hit = true
		w.Write([]byte("internal"))
	}))
	defer srv.Close() // listens on 127.0.0.1

	svc := &mediaService{}
	for _, u := range []string{srv.URL + "/a.png", "http://169.254.169.254/latest/meta-data", "file:///etc/passwd", "ftp://example.com/x"} {
		if _, err := svc.UploadFromURL(context.Background(), uuid.New(), u, "", ""); err == nil {
			t.Errorf("expected %q to be rejected", u)
		}
	}
	if hit {
		t.Fatal("loopback server must never be contacted")
	}
}
