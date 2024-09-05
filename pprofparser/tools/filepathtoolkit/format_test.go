package filepathtoolkit

import (
	"fmt"
	"testing"
)

func TestDirName(t *testing.T) {

	path := "C:\\Users\\zhangyi\\AppData\\Local\\Programs\\Python\\Python310\\lib\\threading.py"

	if DirName(path) != "C:\\Users\\zhangyi\\AppData\\Local\\Programs\\Python\\Python310\\lib" {
		t.Fatal("Dirname")
	}

	fmt.Println(DirName("/root/zy/foo.go"))
	fmt.Println(BaseName("/root/zy/foo.go"))

	fmt.Println(DirName("./foo.go"))

	fmt.Println(DirName("/root"))

	fmt.Println(DirName("C:\\demo.java"))

	fmt.Println(BaseName("C:\\Users\\zhangyi\\AppData\\Local\\Programs\\Python\\Python310\\lib\\threading.py"))

	fmt.Println(DirName("<attrs generated init ddtrace.profiling.collector.stack_event.StackSampleEvent>"))
	fmt.Println(BaseName("<attrs generated init ddtrace.profiling.collector.stack_event.StackSampleEvent>"))

}
