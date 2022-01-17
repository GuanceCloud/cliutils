package logio

import (
	"log"
	"testing"
)

func TestOptionLog(t *testing.T) {
	o := &Option{
		Path:       `/tmp/option-log`,
		Level:      `DEBUG`,
		JSONFormat: true,
		Flags:      log.Ldate | log.Ltime | log.Lmicroseconds | log.LUTC | log.Llongfile,
	}

	if err := o.SetLog(); err != nil {
		t.Fatal(err)
	}

	log.Printf("[debug] option-log: this is test")
	log.Printf("[info] option-log: this is test")
	log.Printf("[warn] option-log: this is test")
}

func TestSetLog(t *testing.T) {
	SetLog(`/tmp/test-set-log`, `debug`, true, false)

	log.Printf("[debug] this is a test log")

	SetLog(`/tmp/test-set-log`, `debug`, false, true)
	log.Printf("[debug] this is a test log with json format")

	SetLog(`/tmp/test-set-log`, `debug`, false, false)
	log.Printf("[debug] this is a test log with json format and long file-name")

	SetLog(`/tmp/test-set-log`, `debug`, false, true)
	log.Printf("[debug] this is a test log with json format and short file-name")
}

func TestNoBackup(t *testing.T) {
	o := Option{
		Path:       `/tmp/option-log`,
		Level:      `DEBUG`,
		JSONFormat: true,
		Flags:      log.Ldate | log.Ltime | log.Lmicroseconds | log.LUTC | log.Llongfile,
		RotateSize: 100,
		Backups:    NoBackUp,
	}

	if err := o.SetLog(); err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 100; i++ {
		log.Printf("[ERROR] log %d error", i)
		log.Printf("[WARN] log %d warn", i)
		i++
	}
}

func TestLog(t *testing.T) {
	o := Option{
		Path:       `/tmp/option-log`,
		Level:      `DEBUG`,
		JSONFormat: true,
		RotateSize: 1024,
	}

	if err := o.SetLog(); err != nil {
		t.Fatal(err)
	}

	log.Printf("[debug] this is a debug message")
	log.Printf("[info] this is a info message")
	log.Printf("[error] this is a error message")
	log.Printf("")
	log.Printf("raw message")

	type X struct {
		a int
		b string
	}

	x := X{
		a: 42,
		b: "hahah",
	}

	log.Printf("%+#v", x)
	log.Printf("[info] %+#v", x)

	o.SetLevel(Info)
	log.Printf("[debug] SHOULD-NOT-LOGGED: %+#v", x)
	log.Printf("SHOULD-NOT-LOGGED: %+#v", x)

	o.SetLevel(Debug)

	log.SetPrefix("{callerxxxx1:tracexxxxx2}")
	log.Printf("[debug] SHOULD-LOGGED: %+#v", x)
	log.Printf("SHOULD-LOGGED: %+#v", x)
	log.SetPrefix("")

	o.RotateSize = 32 * 1024 * 1024
	for i := 0; i < 100; i++ {
		log.Printf("[ERROR] log %d error", i)
		log.Printf("[WARN] log %d warn", i)
		i++
	}
}

func TestJsonFormatLog(t *testing.T) {
	o := Option{
		Path:       `/tmp/option-log`,
		Level:      `DEBUG`,
		JSONFormat: true,
	}

	if err := o.SetLog(); err != nil {
		t.Fatal(err)
	}

	log.Printf("[debug] this is a debug message")
	log.Printf("[info] this is a info message")
	log.SetPrefix("{callerxxxx11:tracexxxxx22}")
	log.Printf("[error] this is a error message")
	log.Printf("")
	log.Printf("raw message")

	type X struct {
		a int
		b string
	}

	x := X{
		a: 42,
		b: "hahah",
	}

	log.Printf("%+#v", x)
	log.Printf("[info] %+#v", x)

	o.SetLevel(Info)
	log.Printf("[debug] SHOULD-NOT-LOGGED: %+#v", x)
	log.Printf("SHOULD-NOT-LOGGED: %+#v", x)

	o.SetLevel(Debug)

	log.Printf("[debug] SHOULD-LOGGED: %+#v", x)
	log.Printf("SHOULD-LOGGED: %+#v", x)
	log.SetPrefix("")
}
