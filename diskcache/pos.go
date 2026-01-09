// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package diskcache

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type pos struct {
	Seek int64  `json:"seek"`
	Name []byte `json:"name"`

	cnt,
	dumpCount int
	dumpInterval time.Duration
	lastDump     time.Time

	fd    *os.File
	fname string        // where to dump the binary data
	buf   *bytes.Buffer // reused buffer to build the binary data
}

func (p *pos) close() error {
	if p.fd != nil {
		if err := p.fd.Close(); err != nil {
			return err
		}

		p.fd = nil
	}

	return nil
}

func (p *pos) String() string {
	if p.Name == nil {
		return fmt.Sprintf(":%d", p.Seek)
	}
	return fmt.Sprintf("%s:%d", string(p.Name), p.Seek)
}

func posFromFile(fname string) (*pos, error) {
	bin, err := os.ReadFile(filepath.Clean(fname))
	if err != nil {
		return nil, err
	}

	if len(bin) <= 8 {
		return nil, nil
	}

	var p pos
	if err := p.UnmarshalBinary(bin); err != nil {
		return nil, err
	}
	return &p, nil
}

func (p *pos) MarshalBinary() ([]byte, error) {
	if p.buf == nil {
		p.buf = new(bytes.Buffer)
	}

	p.buf.Reset()

	if err := binary.Write(p.buf, binary.LittleEndian, p.Seek); err != nil {
		return nil, err
	}

	if _, err := p.buf.Write(p.Name); err != nil {
		return nil, err
	}

	return p.buf.Bytes(), nil
}

func (p *pos) UnmarshalBinary(bin []byte) error {
	p.buf = bytes.NewBuffer(bin)

	if err := binary.Read(p.buf, binary.LittleEndian, &p.Seek); err != nil {
		return err
	}

	p.Name = p.buf.Bytes()
	return nil
}

func (p *pos) reset() error {
	if p.fd == nil {
		if fd, err := os.OpenFile(p.fname, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600); err != nil {
			return fmt.Errorf("open pos file(%q) failed: %w", p.fname, err)
		} else {
			p.fd = fd
		}
	}

	if p.buf != nil {
		p.buf.Reset()
	}

	if p.Name == nil && p.Seek == -1 { // has been reset
		return nil
	}

	p.Seek = -1
	p.Name = nil

	return p.doDumpFile()
}

func (p *pos) doDumpFile() error {
	if p.fd == nil {
		if fd, err := os.OpenFile(p.fname, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600); err != nil {
			return fmt.Errorf("open pos file(%q) failed: %w", p.fname, err)
		} else {
			p.fd = fd
		}
	}

	if data, err := p.MarshalBinary(); err != nil {
		return err
	} else {
		if err := p.fd.Truncate(0); err != nil {
			return fmt.Errorf("fd.Truncate: %w", err)
		}

		if _, err := p.fd.Seek(0, 0); err != nil {
			return fmt.Errorf("fd.Seek: %w", err)
		}

		if _, err := p.fd.Write(data); err != nil {
			return fmt.Errorf("dumpFile(%q): %w", p.fname, err)
		}

		return nil
	}
}

func (p *pos) dumpFile() (bool, error) {
	if p.dumpCount == 0 { // force dump .pos on every Get action.
		return true, p.doDumpFile()
	}

	p.cnt++
	if p.cnt%p.dumpCount == 0 {
		p.lastDump = time.Now()
		return true, p.doDumpFile()
	}

	if p.dumpInterval > 0 {
		if time.Since(p.lastDump) >= p.dumpInterval {
			p.lastDump = time.Now()
			return true, p.doDumpFile()
		}
	}

	return false, nil
}

// for benchmark.
func (p *pos) dumpJSON() ([]byte, error) {
	j, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	return j, nil
}
