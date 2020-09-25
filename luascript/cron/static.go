package luascript

import (
	"strings"

	"github.com/robfig/cron"
	"github.com/yuin/gopher-lua/parse"

	"gitlab.jiagouyun.com/cloudcare-tools/cliutils/logger"
	"gitlab.jiagouyun.com/cloudcare-tools/cliutils/luascript/module"
)

var (
	ll = logger.DefaultSLogger("lua_script")
	// l = logger.SLogger("lua_script")
	specParser = cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month)

	luaCache = &module.LuaCache{}
)

func CheckLuaCode(code string) error {
	reader := strings.NewReader(code)
	_, err := parse.Parse(reader, "<string>")
	return err
}
