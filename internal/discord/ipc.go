package discord

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
)

// Discord IPC opcodes.
const (
	opHandshake = 0
	opFrame     = 1
	opClose     = 2
	opPing      = 3
	opPong      = 4
)

// frameHeaderSize is the size of the opcode+length header on every IPC frame.
const frameHeaderSize = 8

// maxFrameBytes caps an incoming frame, guarding against a misbehaving peer.
const maxFrameBytes = 8 << 20 // 8 MiB

// writeFrame writes a frame whose payload is JSON-encoded.
func writeFrame(w io.Writer, op int, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encoding frame payload: %w", err)
	}
	return writeRawFrame(w, op, data)
}

// writeRawFrame writes a frame with a pre-encoded payload.
func writeRawFrame(w io.Writer, op int, data []byte) error {
	header := make([]byte, frameHeaderSize)
	binary.LittleEndian.PutUint32(header[0:4], uint32(op))
	binary.LittleEndian.PutUint32(header[4:8], uint32(len(data)))

	if err := writeFull(w, header); err != nil {
		return err
	}
	return writeFull(w, data)
}

// readFrame reads one frame and returns its opcode and payload.
func readFrame(r io.Reader) (int, []byte, error) {
	header := make([]byte, frameHeaderSize)
	if _, err := io.ReadFull(r, header); err != nil {
		return 0, nil, err
	}

	op := int(binary.LittleEndian.Uint32(header[0:4]))
	length := binary.LittleEndian.Uint32(header[4:8])
	if length > maxFrameBytes {
		return 0, nil, fmt.Errorf("frame of %d bytes exceeds limit of %d", length, maxFrameBytes)
	}

	data := make([]byte, length)
	if _, err := io.ReadFull(r, data); err != nil {
		return 0, nil, err
	}
	return op, data, nil
}

// writeFull writes all of data, retrying on short writes.
func writeFull(w io.Writer, data []byte) error {
	for len(data) > 0 {
		n, err := w.Write(data)
		if err != nil {
			return err
		}
		data = data[n:]
	}
	return nil
}
