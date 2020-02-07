package p2p

import (
	"encoding/binary"
	"errors"
	"io"
	"time"

	"github.com/libp2p/go-libp2p-core/network"
)

const LengthHeader = 4 // LengthHeader represent how many bytes we used as header
const TimeoutReadHeader = time.Second
const TimeoutReadPayload = time.Second * 2

// ReadLength will read the length from stream
func ReadLength(stream network.Stream) (uint32, error) {
	buf := make([]byte, LengthHeader)
	r := io.LimitReader(stream, LengthHeader)
	if err := stream.SetReadDeadline(time.Now().Add(TimeoutReadHeader)); nil != err {
		if errReset := stream.Reset(); errReset != nil {
			return 0, errReset
		}
		return 0, err
	}
	_, err := r.Read(buf)
	if err != nil && !errors.Is(err, io.EOF) {
		if errReset := stream.Reset(); errReset != nil {
			return 0, errReset
		}
		return 0, err
	}
	return binary.LittleEndian.Uint32(buf), nil
}

// ReadPayload from stream
func ReadPayload(stream network.Stream, length uint32) ([]byte, error) {
	buf := make([]byte, length)
	if err := stream.SetReadDeadline(time.Now().Add(TimeoutReadPayload)); nil != err {
		if errReset := stream.Reset(); errReset != nil {
			return nil, errReset
		}
		return nil, err
	}

	n, err := stream.Read(buf)
	if err != nil && !errors.Is(err, io.EOF) {
		if errReset := stream.Reset(); errReset != nil {
			return nil, errReset
		}
		return nil, err
	}
	return buf, nil
}
