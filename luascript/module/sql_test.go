package module

import (
	"testing"

	lua "github.com/yuin/gopher-lua"
)

func TestSQL(t *testing.T) {
	var luaCode = `
	for i=1, 10000 do
		print("mysql test")
		conn, err = sql_connect("mysql", "root:123456@tcp(10.100.64.106:33306)/db123?charset=utf8")
		if err ~= "" then
			res, err = conn:query("SELECT * FROM students where year>?;", 1986)
			if res ~= nil then
				for _, row in pairs(res) do
					for k, v in pairs(row) do
						print(k, v)
					end
				end
			else
				print(err)
			end
		else
			print(err)
		end
		conn:close()
	end
`
	l := lua.NewState()
	defer l.Close()

	RegisterSQLFuncs(l)
	defer connPool.close()

	if err := l.DoString(luaCode); err != nil {
		t.Error(err)
	}
}
