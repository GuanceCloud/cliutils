package logio

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"path"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	Fatal = 4
	Error = 3
	Warn  = 2
	Info  = 1
	Debug = 0
)

var (
	JSONFormat = "{\"timestamp\":%d, \"level\": \"%s\", \"message\": \"%s\",\"traceId\":\"%s\"}\n"

	levels = map[string]int{

		`[debug]`: Debug,
		`[DEBUG]`: Debug,
		`[info]`:  Info,
		`[INFO]`:  Info,
		`[warn]`:  Warn,
		`[WARN]`:  Warn,
		`[error]`: Error,
		`[ERROR]`: Error,
		`[fatal]`: Fatal,
		`[FATAL]`: Fatal,
	}

	NoBackUp = -1
)

var (
	defaultRotateSize = int64(1024 * 1024 * 32)
	defaultBackups    = 5
	defaultFlags      = log.LstdFlags | log.Llongfile | log.LUTC
)

type Option struct {
	Path  string
	Level string

	JSONFormat bool
	Flags      int
	Backups    int
	RotateSize int64

	rw     *rotateWriter
	format string
}

type rotateWriter struct {
	lock          sync.Mutex
	filename      string
	fp            *os.File
	curBytes      int64
	backups       []string
	format        string
	level         int
	beginAt       time.Time
	disableBackup bool

	o *Option
}

func (o *Option) newWriter() error {
	rw := &rotateWriter{
		filename: o.Path,
		format:   o.format,
		o:        o,
	}

	o.rw = rw

	if err := o.rotate(); err != nil {
		return err
	}
	return nil
}

func (o *Option) SetLog() error {
	if err := os.MkdirAll(path.Dir(o.Path), os.ModePerm); err != nil {
		return err
	}

	if o.JSONFormat {
		o.format = JSONFormat
	}

	if err := o.newWriter(); err != nil {
		return err
	}

	if o.Flags == 0 {
		o.Flags = defaultFlags
	}

	switch o.Backups {
	case 0:
		o.Backups = defaultBackups
	case NoBackUp:
		o.Backups = 0
	}

	if o.RotateSize == 0 {
		o.RotateSize = defaultRotateSize
	}

	log.SetFlags(o.Flags)

	switch strings.ToUpper(o.Level) {
	case `DEBUG`:
		o.SetLevel(Debug)
	case `INFO`:
		o.SetLevel(Info)
	case `WARN`:
		o.SetLevel(Warn)
	case `ERROR`:
		o.SetLevel(Error)
	case `FATAL`:
		o.SetLevel(Fatal)
	default:
		o.SetLevel(Debug)
	}

	log.SetOutput(o.rw)
	return nil
}

func SetLog(f, level string, disableJsonFmt, disableLongFileName bool) {
	defaultOption := &Option{
		Path:       f,
		Level:      level,
		JSONFormat: !disableJsonFmt,
		Flags:      log.LstdFlags | log.Llongfile,
		RotateSize: defaultRotateSize,
		Backups:    defaultBackups,
	}

	if disableLongFileName {
		defaultOption.Flags = log.LstdFlags | log.Lshortfile
	}

	defaultOption.SetLog()
}

func (w *rotateWriter) DisableBackup() {
	w.disableBackup = true
}

func (w *rotateWriter) Close() error {
	return w.fp.Close()
}

func (o *Option) SetLevel(l int) {
	switch l {
	case Fatal, Error, Warn, Info, Debug:
		o.rw.level = l
	default:
		o.rw.level = Debug
	}
}

func (w *rotateWriter) logFiles(all bool) []string {
	if all {
		return append(w.backups, w.filename)
	} else {
		return []string{w.filename}
	}
}

func fmtTs(t time.Time) string {
	return fmt.Sprintf("%04d-%02d-%02d_%02d:%02d:%02d",
		t.Year(), t.Month(), t.Day(),
		t.Hour(), t.Minute(), t.Second())
}

func (o *Option) rotate() error {
	o.rw.lock.Lock()
	defer o.rw.lock.Unlock()

	var err error

	if o.rw.fp != nil {
		err = o.rw.fp.Close()
		o.rw.fp = nil
		if err != nil {
			return err
		}
	}

	if fi, err := os.Stat(o.rw.filename); err == nil {
		if fi.Size() < o.RotateSize { // 继续追加日志
			goto __open_file
		} else {
			// 切分出另一个日志
			backupName := o.rw.filename + `-` + fmtTs(o.rw.beginAt) + `-` + fmtTs(time.Now())
			if err := os.Rename(o.rw.filename, backupName); err != nil {
				return err
			}

			o.rw.backups = append(o.rw.backups, backupName)
			o.rw.beginAt = time.Now()

			if len(o.rw.backups) > o.Backups {
				// 移除 backup 日志

				_ = os.Remove(o.rw.backups[0])
				o.rw.backups = o.rw.backups[1:]
			}
		}
	}

__open_file:
	o.rw.fp, err = os.OpenFile(o.rw.filename, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		return err
	}

	// append 模式下, 当前文件大小计入 curBytes
	if fi, err := os.Stat(o.rw.filename); err == nil {
		o.rw.curBytes = fi.Size()
	} else {
		return err
	}

	return nil
}

func (w *rotateWriter) Write(data []byte) (int, error) {
	if w.curBytes == 0 {
		w.beginAt = time.Now()
	}

	if w.curBytes >= w.o.RotateSize && !w.disableBackup {
		if err := w.o.rotate(); err != nil {
			log.Printf("rotate failed: %s, ignored", err.Error())
		}
		w.curBytes = 0
	}

	w.lock.Lock()
	defer w.lock.Unlock()

	n := 0
	var err error

	level := `debug`
	key := `[debug]`
	lv := Debug

	for k, l := range levels {
		if bytes.Contains(data, []byte(k)) {
			level = k[1 : len(k)-1]
			lv = l
			key = k
			break
		}
	}

	if w.level > lv { // 当前日志被过滤掉
		return 0, nil
	}

	switch w.format {
	case JSONFormat:

		data = data[0 : len(data)-1]                            // 去掉自带的换行
		data = bytes.Replace(data, []byte(key), []byte(``), -1) // 去掉消息体中的 level
		var traceId string
		r := regexp.MustCompile(`(?i)\[traceId:(.+)\]`)
		ts := r.FindAllStringSubmatch(string(data), -1)
		for _, s := range ts {
			data = bytes.Replace(data, []byte(s[0]), []byte(``), -1)
			traceId = s[1]
		}

		n, err = w.fp.Write([]byte(fmt.Sprintf(w.format,
			time.Now().Unix(),
			strings.ToUpper(level),
			data, traceId)))
	default: // 不带任何格式化，则按照默认情况输出
		n, err = w.fp.Write(data)
	}

	if err == nil {
		w.curBytes += int64(n)
	}
	return n, err
}
