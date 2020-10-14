package module

import (
	"testing"

	lua "github.com/yuin/gopher-lua"
)

func TestCache(t *testing.T) {
	var luaCode = `
	print("cache test:")

	cache_set("AAA", "hello,world")
	cache_set("BBB", 123456)
	cache_set("CCC", true)
	cache_set("DDD", { host = '10.100.64.106', port = 13306, database = 'golua_mysql', user = 'root', password = '123456' })

	list = cache_list()
	print("cache_list:")
	for k,v in pairs(list) do
		print(k, v)
	end
	print("cache list len:", #list)
	print("cache list len:", table.getn(list))

	print("AAA: ", cache_get("AAA"))
	print("BBB: ", cache_get("BBB"))
	print("CCC: ", cache_get("CCC"))

	print("DDD --------")
	dd = cache_get("DDD")
	for k,v in pairs(dd) do
		print(k, v)
	end
`
	l := lua.NewState()
	defer l.Close()

	RegisterCacheFuncs(l)
	if err := l.DoString(luaCode); err != nil {
		t.Error(err)
	}
}
