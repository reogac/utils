package httpw

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"syscall"
	"testing"
	"time"
)

// refusing is a dial refused its first n times, then connected through a pipe;
// calls counts every attempt.
func refusing(n int, calls *int) dialFunc {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		*calls++
		if *calls <= n {
			return nil, &net.OpError{Op: "dial", Net: network, Err: fmt.Errorf("connect: %w", syscall.ECONNREFUSED)}
		}
		c, _ := net.Pipe()
		return c, nil
	}
}

func statsSince(before DialStats) DialStats {
	now := ReadDialStats()
	return DialStats{Refused: now.Refused - before.Refused, Failed: now.Failed - before.Failed}
}

// A refusal is answered by a new connection: the next pick behind a Service is
// another replica, and the dial succeeds without its caller seeing the refusal.
func TestARefusedDialIsMadeAgain(t *testing.T) {
	before := ReadDialStats()
	calls := 0
	conn, err := redialRefused(refusing(2, &calls))(context.Background(), "tcp", "10.0.0.1:7777")
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	conn.Close()
	if calls != 3 {
		t.Errorf("attempts: %d, want 3", calls)
	}
	if got := statsSince(before); got != (DialStats{Refused: 2}) {
		t.Errorf("stats: %+v, want 2 refused, none failed", got)
	}
}

// The redials are bounded: a dial refused every time fails after RefusedRedials
// more, with the refusal still its cause, and is counted failed once.
func TestADialRefusedEveryTimeFails(t *testing.T) {
	before := ReadDialStats()
	calls := 0
	_, err := redialRefused(refusing(1<<30, &calls))(context.Background(), "tcp", "10.0.0.1:7777")
	if !errors.Is(err, syscall.ECONNREFUSED) {
		t.Fatalf("err: %v, want a refusal", err)
	}
	if calls != 1+RefusedRedials {
		t.Errorf("attempts: %d, want %d", calls, 1+RefusedRedials)
	}
	if got := statsSince(before); got != (DialStats{Refused: uint64(1 + RefusedRedials), Failed: 1}) {
		t.Errorf("stats: %+v", got)
	}
}

// Only a refusal is redialled: a timeout may mean a peer that is slow, not gone,
// and dialling it again only spends the caller's deadline.
func TestOnlyARefusalIsRedialled(t *testing.T) {
	before := ReadDialStats()
	calls := 0
	timeout := func(ctx context.Context, network, addr string) (net.Conn, error) {
		calls++
		return nil, &net.OpError{Op: "dial", Net: network, Err: context.DeadlineExceeded}
	}
	if _, err := redialRefused(timeout)(context.Background(), "tcp", "10.0.0.1:7777"); err == nil {
		t.Fatal("dial succeeded")
	}
	if calls != 1 {
		t.Errorf("attempts: %d, want 1", calls)
	}
	if got := statsSince(before); got != (DialStats{}) {
		t.Errorf("stats: %+v, want none", got)
	}
}

// A context that ended stops the redials: the request waiting on the dial has
// given up, and the next attempt would be refused its deadline anyway.
func TestAnEndedContextStopsTheRedials(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	dial := func(c context.Context, network, addr string) (net.Conn, error) {
		calls++
		cancel()
		return nil, &net.OpError{Op: "dial", Net: network, Err: syscall.ECONNREFUSED}
	}
	if _, err := redialRefused(dial)(ctx, "tcp", "10.0.0.1:7777"); !errors.Is(err, syscall.ECONNREFUSED) {
		t.Fatalf("err: %v", err)
	}
	if calls != 1 {
		t.Errorf("attempts: %d, want 1", calls)
	}
}

// Both of the client's transports redial: a request to a port nothing listens
// on is refused 1 + RefusedRedials times and fails once, h2c and TLS alike.
func TestTheClientRedialsARefusedConnection(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	l.Close() // nothing listens there now: every dial is refused at once
	for _, c := range []struct {
		name string
		cli  *Client
	}{
		{"h2c", NewClient(nil, nil, "")},
		{"tls", tlsClient(t)},
	} {
		t.Run(c.name, func(t *testing.T) {
			before := ReadDialStats()
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+addr+"/x", nil)
			if _, err := c.cli.SendRequest(req); !errors.Is(err, syscall.ECONNREFUSED) {
				t.Fatalf("err: %v, want a refusal", err)
			}
			if got := statsSince(before); got != (DialStats{Refused: uint64(1 + RefusedRedials), Failed: 1}) {
				t.Errorf("stats: %+v", got)
			}
		})
	}
}

// tlsClient is a TLS client with a throwaway self-signed certificate: the dial
// is refused before any handshake, so the certificate is never looked at.
func tlsClient(t *testing.T) *Client {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{SerialNumber: big.NewInt(1), NotAfter: time.Now().Add(time.Hour), IsCA: true,
		BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	ca, _ := x509.ParseCertificate(der)
	pool := x509.NewCertPool()
	pool.AddCert(ca)
	return NewClient(&tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}, pool, "localhost")
}
