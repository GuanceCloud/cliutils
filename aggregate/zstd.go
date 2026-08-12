package aggregate

import (
	"fmt"
	"sync"

	"github.com/klauspost/compress/zstd"
)

const (
	// PayloadCompressionNone 表示 points_payload 为原始 PBPoints 编码，未压缩。
	PayloadCompressionNone int32 = 0
	// PayloadCompressionZstd 表示 points_payload 为 zstd 压缩的 PBPoints 编码。
	PayloadCompressionZstd int32 = 1
)

// zstdEncoderPool 复用 zstd encoder，避免高频压缩时重复初始化。
var zstdEncoderPool = sync.Pool{
	New: func() any {
		enc, err := zstd.NewWriter(nil)
		if err != nil {
			panic(fmt.Sprintf("new zstd encoder: %v", err))
		}
		return enc
	},
}

// zstdDecoderPool 复用 zstd decoder。
var zstdDecoderPool = sync.Pool{
	New: func() any {
		dec, err := zstd.NewReader(nil)
		if err != nil {
			panic(fmt.Sprintf("new zstd decoder: %v", err))
		}
		return dec
	},
}

// CompressPointsPayload 压缩 PBPoints payload。
// 返回压缩后的字节与压缩方式；若压缩无收益（结果不小于原始大小），
// 返回原始字节与 PayloadCompressionNone。
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

// DecompressPointsPayload 按压缩方式解压 PBPoints payload。
// compression 为 PayloadCompressionNone 时原样返回，不复制。
func DecompressPointsPayload(payload []byte, compression int32) ([]byte, error) {
	if len(payload) == 0 || compression == PayloadCompressionNone {
		return payload, nil
	}

	if compression != PayloadCompressionZstd {
		return nil, fmt.Errorf("unsupported payload compression: %d", compression)
	}

	dec := zstdDecoderPool.Get().(*zstd.Decoder)
	decompressed, err := dec.DecodeAll(payload, nil)
	zstdDecoderPool.Put(dec)
	if err != nil {
		return nil, fmt.Errorf("zstd decode points payload: %w", err)
	}

	return decompressed, nil
}
