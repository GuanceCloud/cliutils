package module

import (
	"testing"

	lua "github.com/yuin/gopher-lua"
)

func TestMongoModule(t *testing.T) {
	var luaCode = `
	for i=1, 10000 do
		print("mongo test")
		conn = mongo_connect("mongodb://10.100.64.106:30002/?connect=direct")

		res, err = conn:query("db123","tb123", {name="y3333"})
		if err == nil then
			print(res._id)
			print(res.name)
		else
			print(err)
		end

		conn:close()
	end
`
	l := lua.NewState()
	defer l.Close()

	RegisterMongoFuncs(l)
	defer connPool.close()

	if err := l.DoString(luaCode); err != nil {
		t.Error(err)
	}
}
