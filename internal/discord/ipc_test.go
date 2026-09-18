package discord

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestFrameRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	if err := writeFrame(&buf, opFrame, map[string]any{"hello": "world"}); err != nil {
		t.Fatalf("writeFrame: %v", err)
	}

	op, data, err := readFrame(&buf)
	if err != nil {
		t.Fatalf("readFrame: %v", err)
	}
	if op != opFrame {
		t.Errorf("op = %d, want %d", op, opFrame)
	}
	if string(data) != `{"hello":"world"}` {
		t.Errorf("payload = %s", data)
	}
}

func TestReadFrameRejectsOversizedFrame(t *testing.T) {
	header := make([]byte, frameHeaderSize)
	binary.LittleEndian.PutUint32(header[0:4], uint32(opFrame))
	binary.LittleEndian.PutUint32(header[4:8], uint32(maxFrameBytes+1))

	if _, _, err := readFrame(bytes.NewReader(header)); err == nil {
		t.Fatal("expected error for oversized frame, got nil")
	}
}
