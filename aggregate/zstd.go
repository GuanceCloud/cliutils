package aggregate

import (
	"errors"
	"fmt"
	"sync"

	"github.com/klauspost/compress/zstd"
)

const (
	// PayloadCompressionNone means points_payload is raw PBPoints encoding (uncompressed).
	PayloadCompressionNone int32 = 0
	// PayloadCompressionZstd means points_payload is zstd-compressed PBPoints encoding.
	PayloadCompressionZstd int32 = 1

	zstdDecoderMaxWindowBytes = 64 << 20
	zstdDecoderMaxMemoryBytes = 256 << 20
)

// ErrUnsupportedPayloadCompression indicates that a DataPacket uses a
// points_payload compression method this cliutils version cannot decode.
var ErrUnsupportedPayloadCompression = errors.New("unsupported payload compression")

// ErrPayloadDecodedSizeUnknown indicates that a compressed frame does not
// advertise its decoded size, so callers cannot reserve bounded memory first.
var ErrPayloadDecodedSizeUnknown = errors.New("payload decoded size is unknown")

func unsupportedPayloadCompressionError(compression int32) error {
	return fmt.Errorf("%w: %d", ErrUnsupportedPayloadCompression, compression)
}

// zstdEncoderPool reuses zstd encoders to avoid re-initialization on hot paths.
var zstdEncoderPool = sync.Pool{
	New: func() any {
		enc, err := zstd.NewWriter(nil,
			zstd.WithEncoderConcurrency(1),
			zstd.WithEncoderLevel(zstd.SpeedFastest),
		)
		if err != nil {
			panic(fmt.Sprintf("new zstd encoder: %v", err))
		}
		return enc
	},
}

// zstdDecoderPool reuses zstd decoders.
var zstdDecoderPool = sync.Pool{
	New: func() any {
		dec, err := zstd.NewReader(nil,
			zstd.WithDecoderConcurrency(1),
			zstd.WithDecoderLowmem(true),
			zstd.WithDecoderMaxWindow(zstdDecoderMaxWindowBytes),
			zstd.WithDecoderMaxMemory(zstdDecoderMaxMemoryBytes),
		)
		if err != nil {
			panic(fmt.Sprintf("new zstd decoder: %v", err))
		}
		return dec
	},
}

// CompressPointsPayload compresses a PBPoints payload.
// It returns the compressed bytes together with the compression method;
// when compression has no benefit (result not smaller than the input),
// the original bytes and PayloadCompressionNone are returned.
func CompressPointsPayload(payload []byte) ([]byte, int32, error) {
	if len(payload) == 0 {
		return payload, PayloadCompressionNone, nil
	}

	enc := zstdEncoderPool.Get().(*zstd.Encoder)
	compressed := enc.EncodeAll(payload, nil)
	zstdEncoderPool.Put(enc)

	if len(compressed) >= len(payload) {
		return payload, PayloadCompressionNone, nil
	}

	return compressed, PayloadCompressionZstd, nil
}

// DecompressPointsPayload decompresses a PBPoints payload by the given compression method.
// PayloadCompressionNone returns the input as-is without copying.
func DecompressPointsPayload(payload []byte, compression int32) ([]byte, error) {
	if len(payload) == 0 || compression == PayloadCompressionNone {
		return payload, nil
	}

	if compression != PayloadCompressionZstd {
		return nil, unsupportedPayloadCompressionError(compression)
	}

	dec := zstdDecoderPool.Get().(*zstd.Decoder)
	decompressed, err := dec.DecodeAll(payload, nil)
	zstdDecoderPool.Put(dec)
	if err != nil {
		return nil, fmt.Errorf("zstd decode points payload: %w", err)
	}

	return decompressed, nil
}

// PointsPayloadDecodedSize returns the number of bytes produced by decoding a
// points_payload without allocating the decoded output. Zstd frames without a
// content-size field are rejected so memory-bounded callers can fail before
// decompression.
func PointsPayloadDecodedSize(payload []byte, compression int32) (int64, error) {
	if len(payload) == 0 || compression == PayloadCompressionNone {
		return int64(len(payload)), nil
	}
	if compression != PayloadCompressionZstd {
		return 0, unsupportedPayloadCompressionError(compression)
	}

	var header zstd.Header
	if err := header.Decode(payload); err != nil {
		return 0, fmt.Errorf("decode zstd points payload header: %w", err)
	}
	if !header.HasFCS {
		return 0, ErrPayloadDecodedSizeUnknown
	}
	if header.FrameContentSize > uint64(1<<63-1) {
		return 0, fmt.Errorf("points payload decoded size overflows int64: %d", header.FrameContentSize)
	}
	return int64(header.FrameContentSize), nil
}
