package logging

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//////
// Tests.
//////

func TestGet(t *testing.T) {
	tests := []struct {
		name       string
		goroutines int
	}{
		{
			name:       "concurrent first access returns one logger",
			goroutines: 50,
		},
		{
			name:       "initialized singleton remains stable",
			goroutines: 50,
		},
		{
			name:       "single caller gets singleton",
			goroutines: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loggers := make([]*Logger, tt.goroutines)
			start := make(chan struct{})

			var wg sync.WaitGroup
			wg.Add(tt.goroutines)

			for i := range loggers {
				go func(index int) {
					defer wg.Done()

					<-start

					loggers[index] = Get()
				}(i)
			}

			close(start)
			wg.Wait()

			require.NotNil(t, loggers[0])

			for _, logger := range loggers[1:] {
				assert.Same(t, loggers[0], logger)
			}
		})
	}
}
