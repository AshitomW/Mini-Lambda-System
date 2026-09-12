package runner_test

import (
	"bytes"
	"encoding/binary"
	"testing"

	"AshitomW/mini-lambda/internal/runner"
)

func TestDemuxStream(t *testing.T) {
	var input bytes.Buffer

	writeFrame := func(streamType byte, payload string) {
		header := make([]byte, 8)
		header[0] = streamType
		binary.BigEndian.PutUint32(header[4:8], uint32(len(payload)))
		input.Write(header)
		input.WriteString(payload)
	}

	writeFrame(1, "hello stdout\n")
	writeFrame(2, "warning stderr\n")
	writeFrame(1, "more stdout\n")

	var stdoutBuf bytes.Buffer
	var stderrBuf bytes.Buffer

	err := runner.DemuxStream(&input, &stdoutBuf, &stderrBuf)
	if err != nil {
		t.Fatalf("unexpected error from DemuxStream: %v", err)
	}

	expectedStdout := "hello stdout\nmore stdout\n"
	if stdoutBuf.String() != expectedStdout {
		t.Errorf("expected stdout %q, got %q", expectedStdout, stdoutBuf.String())
	}

	expectedStderr := "warning stderr\n"
	if stderrBuf.String() != expectedStderr {
		t.Errorf("expected stderr %q, got %q", expectedStderr, stderrBuf.String())
	}
}
