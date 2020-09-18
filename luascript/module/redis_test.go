package module

import (
	"testing"

	lua "github.com/yuin/gopher-lua"
)

func TestRedis(t *testing.T) {
	var luaCode = `
	for i=1, 10000 do
		print("redis test")

		conn = redis_connect({host="10.100.64.106", port=36379, passwd="", index=0})

		-- print(conn:docmd("set", "testkey", "testvalue"))
		-- res, err = conn:docmd("keys", "wallet*")
		-- if err == nil then
		-- 	for k, v in ipairs(res) do
		-- 		print(k, v)
		-- 	end
		-- else
		-- 	print(err)
		-- end

		print(conn:docmd("get", "testkey"))

		conn:close()
	end
`
	l := lua.NewState()
	defer l.Close()

	RegisterRedisFuncs(l)
	defer connPool.close()

	if err := l.DoString(luaCode); err != nil {
		t.Error(err)
	}
}
