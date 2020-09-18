package module

import (
	"sync"

	lua "github.com/yuin/gopher-lua"
)

type luaCache struct {
	sync.Map
}

func (c *luaCache) get(L *lua.LState) int {
	key := L.ToString(1)
	value, ok := c.Load(key)
	if !ok {
		L.Push(lua.LNil)
		return 1
	}

	switch value.(type) {
	case lua.LNumber:
		L.Push(value.(lua.LNumber))
	case lua.LString:
		L.Push(value.(lua.LString))
	case lua.LBool:
		L.Push(value.(lua.LBool))
	case *lua.LTable:
		L.Push(value.(*lua.LTable))
	default:
		L.Push(lua.LNil)
	}

	return 1
}

func (c *luaCache) set(L *lua.LState) int {
	c.Store(L.ToString(1), L.Get(2))
	return 0
}

func (c *luaCache) list(L *lua.LState) int {
	var list []string
	c.Range(func(key, value interface{}) bool {
		list = append(list, key.(string))
		return true
	})

	tb := L.NewTable()
	for _, v := range list {
		tb.Append(lua.LString(v))
	}
	L.Push(tb)
	return 1
}
