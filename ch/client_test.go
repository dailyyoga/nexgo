package ch

import (
	"context"
	"errors"
	"testing"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

// pingStubConn is a driver.Conn whose Ping returns a configured error. The other
// methods come from the embedded nil interface and must not be called by Ping.
type pingStubConn struct {
	driver.Conn
	err error
}

func (c pingStubConn) Ping(context.Context) error { return c.err }

func TestClientPing(t *testing.T) {
	c := &defaultClient{conn: pingStubConn{}}

	// Healthy connection.
	if err := c.Ping(context.Background()); err != nil {
		t.Errorf("Ping on healthy conn = %v, want nil", err)
	}

	// Underlying ping error propagates.
	wantErr := errors.New("clickhouse down")
	c.conn = pingStubConn{err: wantErr}
	if err := c.Ping(context.Background()); !errors.Is(err, wantErr) {
		t.Errorf("Ping error = %v, want %v", err, wantErr)
	}

	// Closed client short-circuits before touching the connection.
	c.closed = true
	if err := c.Ping(context.Background()); !errors.Is(err, ErrConnectionClosed) {
		t.Errorf("Ping after close = %v, want ErrConnectionClosed", err)
	}
}
