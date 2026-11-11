package httpw

import (
	"context"
	"errors"
	"net"
	"sync/atomic"
	"syscall"
)

// RefusedRedials is how many times a dial whose connection was refused is made
// again, to the same address, before the refusal is returned. The address a
// client dials is often a Service's, behind which kube-proxy picks a backend for
// each new connection, and a replica that just died stays listed for up to a
// second: a new connection is a new pick, and one of the live replicas answers
// it. Nothing was written on a refused connection, so redialling cannot
// duplicate a request, whatever its method.
const RefusedRedials = 3

// The redial sits in the dial and not around the request because the HTTP/2
// pool dials once for every request waiting on a connection to that address and
// hands each of them the dial's error: one refusal fails them all, and one
// redial answers them all.

var dialsRefused, dialsFailed atomic.Uint64

// DialStats counts the dials of every Client in the process since it started.
type DialStats struct {
	//Refused counts connection attempts the peer refused, each redial included
	Refused uint64
	//Failed counts dials that were still refused after every redial, or whose
	//context ended between two: each failed every request waiting on it
	Failed uint64
}

// ReadDialStats returns the process's dial counts, for a metrics collector.
func ReadDialStats() DialStats {
	return DialStats{Refused: dialsRefused.Load(), Failed: dialsFailed.Load()}
}

type dialFunc func(ctx context.Context, network, addr string) (net.Conn, error)

// redialRefused makes dial again, at once, when its connection is refused. A
// refusal comes back within one round trip, so the redials cost that much each;
// to an address with nothing behind it that is all they cost. No pause between
// them: the next pick is no likelier to be a live replica after one.
func redialRefused(dial dialFunc) dialFunc {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		for n := 0; ; n++ {
			conn, err := dial(ctx, network, addr)
			if err == nil || !errors.Is(err, syscall.ECONNREFUSED) {
				return conn, err
			}
			dialsRefused.Add(1)
			if n == RefusedRedials || ctx.Err() != nil {
				dialsFailed.Add(1)
				return nil, err
			}
		}
	}
}
