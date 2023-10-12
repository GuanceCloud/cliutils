// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

// Package http wraps all HTTP releated common-used utils.
package http

import (
	"net/http"
)

var (
	// ZIPMagic see https://en.wikipedia.org/wiki/ZIP_(file_format)#Local_file_header
	ZIPMagic = []byte{0x50, 0x4b, 0x3, 0x4} //

	// LZ4Magic see https://android.googlesource.com/platform/external/lz4/+/HEAD/doc/lz4_Frame_format.md#general-structure-of-lz4-frame-format
	LZ4Magic = []byte{0x4, 0x22, 0x4d, 0x18}

	// GzipMagic see https://en.wikipedia.org/wiki/Gzip#File_format
	GzipMagic = []byte{0x1f, 0x8b}
)

// ReadBody will automatically unzip the body, it doesn't close the Request.Body.
func ReadBody(req *http.Request) ([]byte, error) {
	body, _, err := gzipReadMD5AndClose(req, false, false)
	return body, err
}
