package luascript

import (
	"testing"
	"time"
)

func TestCronLua(t *testing.T) {
	luaCode := `
	print("---------- lua ---------")
	`
	c := NewLuaCron()
	err := c.AddHandle(luaCode, "*/1 * * * *")
	if err != nil {
		t.Fatal(err)
	}
	c.Run()
	defer c.Stop()

	time.Sleep(time.Second * 5)
}
