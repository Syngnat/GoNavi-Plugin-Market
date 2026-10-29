package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type wsServerOptions struct {
	status string
	accept func(string) string
	frames func(net.Conn, *bufio.Reader)
}

// startWebSocketServer hand-rolls the server half of the RFC 6455 handshake so
// the client can be tested without a third-party WebSocket library.
func startWebSocketServer(t *testing.T, options wsServerOptions) (string, chan struct{}) {
	t.Helper()
	done := make(chan struct{})
	status := options.status
	if status == "" {
		status = "101 Switching Protocols"
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(done)
		hijacker, ok := w.(http.Hijacker)
		if !ok {
			t.Error("response writer cannot be hijacked")
			return
		}
		conn, buffer, err := hijacker.Hijack()
		if err != nil {
			t.Error(err)
			return
		}
		defer func() { _ = conn.Close() }()
		accept := wsAcceptValue(r.Header.Get("Sec-WebSocket-Key"))
		if options.accept != nil {
			accept = options.accept(r.Header.Get("Sec-WebSocket-Key"))
		}
		response := "HTTP/1.1 " + status + "\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n" +
			"Sec-WebSocket-Accept: " + accept + "\r\n\r\n"
		if _, err := io.WriteString(conn, response); err != nil {
			return
		}
		if options.frames != nil {
			options.frames(conn, buffer.Reader)
		}
	}))
	t.Cleanup(server.Close)
	return "ws://" + strings.TrimPrefix(server.URL, "http://") + "/devtools/page/T1", done
}

func TestDialWebSocketRejectsWrongAccept(t *testing.T) {
	url, done := startWebSocketServer(t, wsServerOptions{accept: func(string) string { return "wrong" }})
	_, err := dialWebSocket(url)
	if !errors.Is(err, errWebSocketHandshake) {
		t.Fatalf("expected a rejected handshake, got %v", err)
	}
	<-done
}

func TestDialWebSocketRejectsNonSwitchingStatus(t *testing.T) {
	url, done := startWebSocketServer(t, wsServerOptions{status: "400 Bad Request"})
	_, err := dialWebSocket(url)
	if !errors.Is(err, errWebSocketHandshake) {
		t.Fatalf("expected a rejected handshake, got %v", err)
	}
	<-done
}

func TestDialWebSocketRejectsNonWebSocketURL(t *testing.T) {
	if _, err := dialWebSocket("http://127.0.0.1:1/devtools/page/T1"); !errors.Is(err, errWebSocketHandshake) {
		t.Fatalf("expected a rejected handshake, got %v", err)
	}
}

// TestWebSocketRoundTripsTextFrames covers both length encodings: a short
// payload uses the 7-bit length and a payload above 125 bytes uses 16-bit.
func TestWebSocketRoundTripsTextFrames(t *testing.T) {
	for _, size := range []int{16, 300} {
		echo := make([]byte, size)
		for index := range echo {
			echo[index] = byte('a' + index%26)
		}
		url, done := startWebSocketServer(t, wsServerOptions{frames: func(conn net.Conn, reader *bufio.Reader) {
			opcode, payload := readMaskedFrame(t, reader)
			if opcode != wsOpcodeText {
				t.Errorf("expected a text frame, got opcode %d", opcode)
			}
			if size > 125 && len(payload) <= 125 {
				t.Errorf("expected the extended length branch for %d bytes", size)
			}
			if err := writeServerFrame(conn, wsOpcodeText, payload); err != nil {
				t.Error(err)
			}
		}})
		conn, err := dialWebSocket(url)
		if err != nil {
			t.Fatalf("size %d: %v", size, err)
		}
		if err := conn.writeFrame(wsOpcodeText, echo); err != nil {
			t.Fatalf("size %d: %v", size, err)
		}
		payload, err := conn.readText()
		if err != nil {
			t.Fatalf("size %d: %v", size, err)
		}
		if !bytes.Equal(payload, echo) {
			t.Fatalf("size %d: echoed payload differs", size)
		}
		_ = conn.Close()
		<-done
	}
}

// TestWebSocketReadsExtendedLengthFrames exercises the client's 64-bit branch.
func TestWebSocketReadsExtendedLengthFrames(t *testing.T) {
	large := bytes.Repeat([]byte("x"), 70000)
	url, done := startWebSocketServer(t, wsServerOptions{frames: func(conn net.Conn, reader *bufio.Reader) {
		readMaskedFrame(t, reader)
		if err := writeServerFrame(conn, wsOpcodeText, large); err != nil {
			t.Error(err)
		}
	}})
	conn, err := dialWebSocket(url)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	if err := conn.writeFrame(wsOpcodeText, []byte("read")); err != nil {
		t.Fatal(err)
	}
	payload, err := conn.readText()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(payload, large) {
		t.Fatalf("expected %d bytes, got %d", len(large), len(payload))
	}
	<-done
}

// TestWebSocketRejectsMaskedServerFrame keeps a masked server frame out: only
// clients mask, so such a frame is a protocol violation.
func TestWebSocketRejectsMaskedServerFrame(t *testing.T) {
	url, done := startWebSocketServer(t, wsServerOptions{frames: func(conn net.Conn, reader *bufio.Reader) {
		readMaskedFrame(t, reader)
		_ = writeMaskedServerFrame(conn, wsOpcodeText, []byte("masked"))
	}})
	conn, err := dialWebSocket(url)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	if err := conn.writeFrame(wsOpcodeText, []byte("hi")); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.readText(); !errors.Is(err, errWebSocketProtocol) {
		t.Fatalf("expected a protocol violation, got %v", err)
	}
	<-done
}

func TestWebSocketAnswersPingAndEndsOnClose(t *testing.T) {
	url, done := startWebSocketServer(t, wsServerOptions{frames: func(conn net.Conn, reader *bufio.Reader) {
		readMaskedFrame(t, reader)
		if err := writeServerFrame(conn, wsOpcodePing, []byte("p")); err != nil {
			t.Error(err)
		}
		opcode, payload := readMaskedFrame(t, reader)
		if opcode != wsOpcodePong || string(payload) != "p" {
			t.Errorf("expected a pong carrying the ping payload, got %d %q", opcode, payload)
		}
		_ = writeServerFrame(conn, wsOpcodeClose, nil)
	}})
	conn, err := dialWebSocket(url)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	if err := conn.writeFrame(wsOpcodeText, []byte("hi")); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.readText(); !errors.Is(err, io.EOF) {
		t.Fatalf("expected end of stream after a close frame, got %v", err)
	}
	<-done
}

func readMaskedFrame(t *testing.T, reader *bufio.Reader) (byte, []byte) {
	t.Helper()
	header := make([]byte, 2)
	if _, err := io.ReadFull(reader, header); err != nil {
		t.Error(err)
		return 0, nil
	}
	if header[1]&0x80 == 0 {
		t.Error("client frames must set the mask bit")
	}
	length := uint64(header[1] & 0x7f)
	switch length {
	case 126:
		extended := make([]byte, 2)
		if _, err := io.ReadFull(reader, extended); err != nil {
			t.Error(err)
			return 0, nil
		}
		length = uint64(binary.BigEndian.Uint16(extended))
	case 127:
		extended := make([]byte, 8)
		if _, err := io.ReadFull(reader, extended); err != nil {
			t.Error(err)
			return 0, nil
		}
		length = binary.BigEndian.Uint64(extended)
	}
	mask := make([]byte, 4)
	if _, err := io.ReadFull(reader, mask); err != nil {
		t.Error(err)
		return 0, nil
	}
	payload := make([]byte, length)
	if _, err := io.ReadFull(reader, payload); err != nil {
		t.Error(err)
		return 0, nil
	}
	for index := range payload {
		payload[index] ^= mask[index%4]
	}
	return header[0] & 0x0f, payload
}

func writeServerFrame(conn net.Conn, opcode byte, payload []byte) error {
	return writeServerFrameWithMask(conn, opcode, payload, false)
}

func writeMaskedServerFrame(conn net.Conn, opcode byte, payload []byte) error {
	return writeServerFrameWithMask(conn, opcode, payload, true)
}

func writeServerFrameWithMask(conn net.Conn, opcode byte, payload []byte, masked bool) error {
	frame := []byte{0x80 | opcode}
	maskBit := byte(0)
	if masked {
		maskBit = 0x80
	}
	switch {
	case len(payload) < 126:
		frame = append(frame, byte(len(payload))|maskBit)
	case len(payload) <= 0xffff:
		frame = append(frame, 126|maskBit)
		frame = binary.BigEndian.AppendUint16(frame, uint16(len(payload)))
	default:
		frame = append(frame, 127|maskBit)
		frame = binary.BigEndian.AppendUint64(frame, uint64(len(payload)))
	}
	if masked {
		mask := []byte{1, 2, 3, 4}
		frame = append(frame, mask...)
		start := len(frame)
		frame = append(frame, payload...)
		for index := range frame[start:] {
			frame[start+index] ^= mask[index%4]
		}
		_, err := conn.Write(frame)
		return err
	}
	frame = append(frame, payload...)
	_, err := conn.Write(frame)
	return err
}
