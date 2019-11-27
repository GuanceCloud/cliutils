package logio

import (
	"log"
	"testing"
)

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
	lw, err := New(`z.log`, JsonFormat)
	if err != nil {
		log.Fatal(err)
	}
	defer lw.Close()

	log.SetOutput(lw)
	log.SetFlags(log.Llongfile)

	lw.SetLevel(Info)
	lw.DisableBackup()

	RotateSize = 32 * 1024 * 1024
	i := 0
	for {
		log.Printf("[ERROR] log %d error", i)
		log.Printf("[WARN] log %d warn", i)
		i++
	}
}

func TestLog(t *testing.T) {
	lw, err := New(`y.log`, "")
	if err != nil {
		log.Fatal(err)
	}
	defer lw.Close()

	log.SetOutput(lw)
	log.SetFlags(log.Llongfile | log.LstdFlags)

	log.SetPrefix("{callerxxxx:tracexxxxx}")
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

	lw.SetLevel(Info)
	log.Printf("[debug] SHOULD-NOT-LOGGED: %+#v", x)
	log.Printf("SHOULD-NOT-LOGGED: %+#v", x)

	lw.SetLevel(Debug)

	log.Printf("[debug] SHOULD-LOGGED: %+#v", x)
	log.Printf("SHOULD-LOGGED: %+#v", x)

	// RotateSize = 32 * 1024 * 1024
	// i := 0
	// for {
	// 	log.Printf("[ERROR] log %d error", i)
	// 	log.Printf("[WARN] log %d warn", i)
	// 	i++
	// }
}

func TestJsonFormatLog(t *testing.T) {

	lw, err := New(`x.log`, JsonFormat)
	if err != nil {
		log.Fatal(err)
	}
	defer lw.Close()

	log.SetOutput(lw)
	log.SetPrefix("{callerxxxx:tracexxxxx}")
	log.SetFlags(log.Llongfile)

	log.Printf("[debug] this is a debug message")
	log.Printf("[info] this is a info message")
	log.SetPrefix("{callerxxxx1:tracexxxxx2}")
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

	lw.SetLevel(Info)
	log.Printf("[debug] SHOULD-NOT-LOGGED: %+#v", x)
	log.Printf("SHOULD-NOT-LOGGED: %+#v", x)

	lw.SetLevel(Debug)

	log.Printf("[debug] SHOULD-LOGGED: %+#v", x)
	log.Printf("SHOULD-LOGGED: %+#v", x)

	// RotateSize = 32 * 1024 * 1024
	// i := 0
	// for {
	// 	log.Printf("[ERROR] log %d error", i)
	// 	log.Printf("[WARN] log %d warn", i)
	// 	i++
	// }
}
