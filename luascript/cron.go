package luascript

import (
	"github.com/robfig/cron"
	lua "github.com/yuin/gopher-lua"

	"gitlab.jiagouyun.com/cloudcare-tools/cliutils/luascript/module"
)

type LuaCron struct {
	*cron.Cron
}

func NewLuaCron() *LuaCron {
	return &LuaCron{
		cron.New(),
	}
}

func (c *LuaCron) AddHandle(code string, intervalSpec string) error {
	if err := CheckLuaCode(code); err != nil {
		return err
	}

	luastate := lua.NewState()
	module.RegisterAllFuncs(luastate, luaCache, nil)

	return c.AddFunc(intervalSpec, func() {
		luastate.DoString(code)
	})
}

func (c *LuaCron) Run() {
	c.Start()
}

func (c *LuaCron) Stop() {
	c.Stop()
}
