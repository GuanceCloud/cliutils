package logio

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"path"
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
	JsonFormat = "{\"timestamp\":%d, \"level\": \"%s\", \"message\": \"%s\",\"traceId\":\"%s\", \"caller\":\"%s\"}\n"

	levels = map[string]int{

		`[debug] `: Debug,
		`[DEBUG] `: Debug,
		`[info] `:  Info,
		`[INFO] `:  Info,
		`[warn] `:  Warn,
		`[WARN] `:  Warn,
		`[error] `: Error,
		`[ERROR] `: Error,
		`[fatal] `: Fatal,
		`[FATAL] `: Fatal,
	}
)

var (
	RotateSize = int64(1024 * 1024 * 32)
	Backups    = 5
)

type RotateWriter struct {
	lock          sync.Mutex
	filename      string
	fp            *os.File
	curBytes      int64
	backups       []string
	format        string
	level         int
	beginAt       time.Time
	disableBackup bool
}

func New(f, format string) (*RotateWriter, error) {
	w := &RotateWriter{filename: f, format: format}

	if err := w.rotate(); err != nil {
		return nil, err
	}
	return w, nil
}

func SetLog(f, level string, disableJsonFmt, disableLongFileName bool) {

	if err := os.MkdirAll(path.Dir(f), os.ModePerm); err != nil {
		log.Fatal(err)
	}

	format := JsonFormat
	if disableJsonFmt {
		format = ""
	}

	rw, err := New(f, format)
	if err != nil {
		log.Fatal(err)
	}

	logFlags := log.LstdFlags | log.Llongfile

	if disableLongFileName {
		logFlags = log.LstdFlags | log.Lshortfile
	}

	log.SetFlags(logFlags)

	switch strings.ToUpper(level) {
	case `DEBUG`:
		rw.SetLevel(Debug)
	case `INFO`:
		rw.SetLevel(Info)
	case `WARN`:
		rw.SetLevel(Warn)
	case `ERROR`:
		rw.SetLevel(Error)
	case `FATAL`:
		rw.SetLevel(Fatal)
	default:
		rw.SetLevel(Debug)
	}

	log.SetOutput(rw)
}

func (w *RotateWriter) DisableBackup() {
	w.disableBackup = true
}

func (w *RotateWriter) Close() error {
	return w.fp.Close()
}

func (w *RotateWriter) SetLevel(l int) {
	switch l {
	case Fatal, Error, Warn, Info, Debug:
		w.level = l
	default:
		w.level = Debug
	}
}

func (w *RotateWriter) LogFiles(all bool) []string {
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

func (w *RotateWriter) rotate() error {
	w.lock.Lock()
	defer w.lock.Unlock()

	var err error

	if w.fp != nil {
		err = w.fp.Close()
		w.fp = nil
		if err != nil {
			return err
		}
	}

	if fi, err := os.Stat(w.filename); err == nil {
		if fi.Size() < RotateSize { // 继续追加日志
			goto __open_file
		} else {
			// 切分出另一个日志
			backupName := w.filename + `-` + fmtTs(w.beginAt) + `-` + fmtTs(time.Now())
			if err := os.Rename(w.filename, backupName); err != nil {
				return err
			}

			w.backups = append(w.backups, backupName)
			w.beginAt = time.Now()

			if len(w.backups) > Backups {
				// 移除 backup 日志

				_ = os.Remove(w.backups[0])
				w.backups = w.backups[1:]
			}
		}
	}

__open_file:
	w.fp, err = os.OpenFile(w.filename, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		return err
	}

	// append 模式下, 当前文件大小计入 curBytes
	if fi, err := os.Stat(w.filename); err == nil {
		w.curBytes = fi.Size()
	} else {
		return err
	}

	return nil
}

func (w *RotateWriter) Write(data []byte) (int, error) {
	if w.curBytes == 0 {
		w.beginAt = time.Now()
	}

	if w.curBytes >= RotateSize && !w.disableBackup {
		if err := w.rotate(); err != nil {
			log.Printf("rotate failed: %s, ignored", err.Error())
		}
		w.curBytes = 0
	}

	w.lock.Lock()
	defer w.lock.Unlock()

	n := 0
	var err error

	level := `debug`
	key := `[debug] `
	lv := Debug

	for k, l := range levels {
		if bytes.Contains(data, []byte(k)) {
			level = k[1 : len(k)-2]
			lv = l
			key = k
			break
		}
	}

	if w.level > lv { // 当前日志被过滤掉
		return 0, nil
	}

	switch w.format {
	case JsonFormat:
		data = data[0 : len(data)-1]                            // 去掉自带的换行
		data = bytes.Replace(data, []byte(key), []byte(``), -1) // 去掉消息体中的 level
		index := bytes.Index(data, []byte(`}`))                 // 前缀添加 {caller:traceId}
		res := data[1:index]
		rs := bytes.Split(res, []byte(`:`))
		n, err = w.fp.Write([]byte(fmt.Sprintf(w.format,
			time.Now().Unix(),
			strings.ToUpper(level),
			data[index+1:], rs[1], rs[0])))
	default: // 不带任何格式化，则按照默认情况输出
		n, err = w.fp.Write(data)
	}

	if err == nil {
		w.curBytes += int64(n)
	}
	return n, err
}
