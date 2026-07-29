package cmd

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//////
// Bridge readiness.
//////

func TestWaitForBridgeReady(t *testing.T) {
	const (
		delayedStart = 60 * time.Millisecond
		interval     = 10 * time.Millisecond
	)

	tests := []struct {
		name       string
		setup      func(t *testing.T) (string, func())
		timeout    time.Duration
		wantErr    bool
		minElapsed time.Duration
		maxElapsed time.Duration
	}{
		{
			name: "happy path bridge is already listening",
			setup: func(t *testing.T) (string, func()) {
				t.Helper()

				listener, err := net.Listen("tcp", "127.0.0.1:0")
				require.NoError(t, err)

				return listener.Addr().String(), func() {
					require.NoError(t, listener.Close())
				}
			},
			timeout:    time.Second,
			maxElapsed: 250 * time.Millisecond,
		},
		{
			name: "edge case bridge starts listening after a delay",
			setup: func(t *testing.T) (string, func()) {
				t.Helper()

				reservation, err := net.Listen("tcp", "127.0.0.1:0")
				require.NoError(t, err)

				address := reservation.Addr().String()
				require.NoError(t, reservation.Close())

				listenerResult := make(chan struct {
					listener net.Listener
					err      error
				}, 1)

				go func() {
					time.Sleep(delayedStart)

					listener, listenErr := net.Listen("tcp", address)
					listenerResult <- struct {
						listener net.Listener
						err      error
					}{listener: listener, err: listenErr}
				}()

				return address, func() {
					result := <-listenerResult
					require.NoError(t, result.err)
					require.NoError(t, result.listener.Close())
				}
			},
			timeout:    time.Second,
			minElapsed: delayedStart / 2,
			maxElapsed: 500 * time.Millisecond,
		},
		{
			name: "bad path bridge never starts listening",
			setup: func(t *testing.T) (string, func()) {
				t.Helper()

				reservation, err := net.Listen("tcp", "127.0.0.1:0")
				require.NoError(t, err)

				address := reservation.Addr().String()
				require.NoError(t, reservation.Close())

				return address, func() {}
			},
			timeout:    80 * time.Millisecond,
			wantErr:    true,
			minElapsed: 60 * time.Millisecond,
			maxElapsed: 300 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			address, cleanup := tt.setup(t)
			defer cleanup()

			started := time.Now()
			err := waitForBridgeReady(address, tt.timeout, interval)
			elapsed := time.Since(started)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			assert.GreaterOrEqual(t, elapsed, tt.minElapsed)
			assert.Less(t, elapsed, tt.maxElapsed)
		})
	}
}
