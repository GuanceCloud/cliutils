package filepathtoolkit

import "testing"

func TestFileExists(t *testing.T) {
	println(FileExists("/sbin/shutdown"))
	println(FileExists("/etc/tmpfile"))
	println(FileExists("/root"))
}
