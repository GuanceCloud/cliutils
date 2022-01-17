package module

import (
	"log"
	"testing"

	lua "github.com/yuin/gopher-lua"
)

func TestLog(t *testing.T) {
	luaCode := `
	function test()
		print("hello")
	end

	print(test)
	tb = {"Hello","World",a=1,b=2,z=3,x=10,y=20,"Good","Bye"}

	log_info("this is info message", 111, tb)
	log_debug("this is debug message", 222, test)
	log_warn("this is warn message",  333)
	log_error("this is error message", 444)
	`

	l := lua.NewState()
	defer l.Close()

	RegisterLogFuncs(l, log.Writer())
	if err := l.DoString(luaCode); err != nil {
		t.Error(err)
	}
}
