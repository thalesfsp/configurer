package cmd

import (
	"bytes"
	"io"
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const esBufferNegativeControlEnv = "CONFIGURER_ESBUFFER_NEGATIVE_CONTROL"

type concurrentLineBuffer interface {
	Len() int
	ReadString(byte) (string, error)
	Write([]byte) (int, error)
}

func TestSyncBufferConcurrentWriteAndReadString(t *testing.T) {
	buffer := concurrentLineBuffer(new(syncBuffer))
	testName := "synchronized buffer"

	if os.Getenv(esBufferNegativeControlEnv) != "" {
		buffer = new(bytes.Buffer)
		testName = "plain bytes buffer negative control"
	}

	tests := []struct {
		name   string
		buffer concurrentLineBuffer
	}{
		{
			name:   testName,
			buffer: buffer,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			started := make(chan struct{})
			lines := make([]string, 0, 10_000)

			var waitGroup sync.WaitGroup
			waitGroup.Add(2)

			go func() {
				defer waitGroup.Done()
				<-started

				for range 10_000 {
					_, _ = tt.buffer.Write([]byte("line\n"))
				}
			}()

			go func() {
				defer waitGroup.Done()
				<-started

				for range 10_000 {
					if tt.buffer.Len() > 0 {
						line, err := tt.buffer.ReadString('\n')
						if err == nil {
							lines = append(lines, line)
						}
					}
				}
			}()

			close(started)
			waitGroup.Wait()

			if os.Getenv(esBufferNegativeControlEnv) != "" {
				return
			}

			for tt.buffer.Len() > 0 {
				line, err := tt.buffer.ReadString('\n')
				require.NoError(t, err)
				lines = append(lines, line)
			}

			require.Len(t, lines, 10_000)
			for _, line := range lines {
				assert.Equal(t, "line\n", line)
			}
		})
	}
}

//////
// syncBuffer
//////

func TestSyncBufferReadString(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		wantLine      string
		wantErr       error
		wantRemaining string
	}{
		{
			name:          "reads complete line",
			input:         "complete\npartial",
			wantLine:      "complete\n",
			wantRemaining: "partial",
		},
		{
			name:          "preserves partial line on EOF",
			input:         "partial",
			wantLine:      "partial",
			wantErr:       io.EOF,
			wantRemaining: "partial",
		},
		{
			name:    "empty buffer returns EOF",
			wantErr: io.EOF,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buffer := new(syncBuffer)

			_, err := buffer.Write([]byte(tt.input))
			require.NoError(t, err)

			line, err := buffer.ReadString('\n')

			assert.Equal(t, tt.wantLine, line)
			assert.ErrorIs(t, err, tt.wantErr)
			assert.Equal(t, tt.wantRemaining, buffer.readRemaining())
		})
	}
}

//////
// Elasticsearch flushing
//////

func TestFlushESBuffer(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		final         bool
		wantLines     []string
		wantRemaining string
	}{
		{
			name:      "flushes complete lines",
			input:     "first\nsecond\n",
			wantLines: []string{"first\n", "second\n"},
		},
		{
			name:          "preserves partial line during periodic flush",
			input:         "partial",
			wantRemaining: "partial",
		},
		{
			name:          "flushes complete line and preserves trailing partial line",
			input:         "complete\npartial",
			wantLines:     []string{"complete\n"},
			wantRemaining: "partial",
		},
		{
			name:      "final shutdown drain flushes complete and partial lines",
			input:     "complete\npartial",
			final:     true,
			wantLines: []string{"complete\n", "partial"},
		},
		{
			name:      "final shutdown drain flushes partial line",
			input:     "partial",
			final:     true,
			wantLines: []string{"partial"},
		},
		{
			name:  "final shutdown drain handles empty buffer",
			final: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buffer := new(syncBuffer)

			_, err := buffer.Write([]byte(tt.input))
			require.NoError(t, err)

			var (
				lines      []string
				readErrors []error
			)

			flushESBuffer(
				buffer,
				tt.final,
				func(line string) {
					lines = append(lines, line)
				},
				func(_ string, err error) {
					readErrors = append(readErrors, err)
				},
			)

			assert.Equal(t, tt.wantLines, lines)
			assert.Empty(t, readErrors)
			assert.Equal(t, tt.wantRemaining, buffer.readRemaining())
		})
	}
}
