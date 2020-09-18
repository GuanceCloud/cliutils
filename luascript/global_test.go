package luascript

import (
	"testing"
	"time"

	"gitlab.jiagouyun.com/cloudcare-tools/ftagent/cfg"
)

func TestGlobalLua(t *testing.T) {

	// luaCode := `
	// printl("---------- lua ---------")
	// `

	path := "/tmp/"
	file := "dw_test_global.lua"

	// if err := ioutil.WriteFile(path+file, []byte(luaCode), 644); err != nil {
	// 	t.Fatal(err)
	// }

	cfg.DWLuaPath = path
	cfg.Cfg.GlobalLua = append(cfg.Cfg.GlobalLua, cfg.LuaConfig{Path: file, Circle: "*/1 * * * *"})

	cronGlobalLua()

	time.Sleep(5 * time.Second)
	// os.Remove(path + file)
}
