// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package aggregate_v2

import (
	"errors"
	"io"
	"sync"

	"github.com/klauspost/compress/zstd"
)

var (
	zstdDecoderPool = sync.Pool{
		New: func() any {
			dec, _ := zstd.NewReader(nil, zstd.WithDecoderConcurrency(1), zstd.WithDecoderMaxWindow(64<<20))
			return dec
		},
	}
	zstdEncoderPool = sync.Pool{
		New: func() any {
			enc, _ := zstd.NewWriter(nil, zstd.WithEncoderConcurrency(1), zstd.WithEncoderLevel(zstd.SpeedFastest))
			return enc
		},
	}
)

// ConcatZstdPayloads merges src into dst directly via RFC 8878 Frame Concatenation.
// When both sides are zstd compressed, it performs 0 decompression and 0 re-compression.
func ConcatZstdPayloads(dst, src []byte) []byte {
	if len(src) == 0 {
		return dst
	}
	if len(dst) == 0 {
		return append([]byte(nil), src...)
	}
	return append(dst, src...)
}

// CompressPayload compresses raw points payload using zstd.
func CompressPayload(raw []byte) ([]byte, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	enc := zstdEncoderPool.Get().(*zstd.Encoder)
	defer zstdEncoderPool.Put(enc)

	compressed := enc.EncodeAll(raw, make([]byte, 0, len(raw)/2+64))
	return compressed, nil
}

// DecompressPayload decompresses zstd payload (supports concatenated multi-frame streams).
func DecompressPayload(compressed []byte) ([]byte, error) {
	if len(compressed) == 0 {
		return nil, nil
	}
	dec := zstdDecoderPool.Get().(*zstd.Decoder)
	defer zstdDecoderPool.Put(dec)

	if err := dec.Reset(nil); err != nil {
		return nil, err
	}

	decompressed, err := dec.DecodeAll(compressed, nil)
	if err != nil {
		return nil, err
	}
	return decompressed, nil
}

// DecodeAllConcatenatedFrames reads all concatenated zstd frames into dst.
func DecodeAllConcatenatedFrames(compressed []byte, dst []byte) ([]byte, error) {
	if len(compressed) == 0 {
		return dst, nil
	}
	dec := zstdDecoderPool.Get().(*zstd.Decoder)
	defer zstdDecoderPool.Put(dec)

	if err := dec.Reset(nil); err != nil {
		return nil, err
	}

	result, err := dec.DecodeAll(compressed, dst[:0])
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	return result, nil
}

// CompressIfNeeded takes a DataPacket's payload and ensures it is zstd compressed.
func CompressIfNeeded(payload []byte, compression int32) ([]byte, int32, error) {
	if len(payload) == 0 {
		return nil, PayloadCompressionNone, nil
	}
	if compression == PayloadCompressionZstd {
		return payload, PayloadCompressionZstd, nil
	}
	compressed, err := CompressPayload(payload)
	if err != nil {
		return nil, 0, err
	}
	return compressed, PayloadCompressionZstd, nil
}
