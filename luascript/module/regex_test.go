package module

import (
	"testing"

	lua "github.com/yuin/gopher-lua"
)

func TestRegexModule(t *testing.T) {
	lucCode := `
	print("regex test:")

	-- quote ----------

	print(re_quote("^$.?a"))
	-- "\^\$\.\?a"

	-- find ----------

	print(re_find('', ''))
	-- 1   0

	print(re_find("abcd efgh ijk", "cd e", 1, true))
	-- 3   6

	print(re_find("abcd efgh ijk", "cd e", -1, true))
	-- nil

	print(re_find("abcd efgh ijk", "i([jk])"))
	-- 11  12  "j"

	-- gsub ----------

	print(re_gsub("hello world", [[(\w+)]], "${1} ${1}"))
	-- "hello hello world world"  2

	print(re_gsub("name version", [[\w+]], {name="lua", version="5.1"}))
	-- "lua-5.1.tar.gz"  2

	print(re_gsub("name version", [[\w+]], {name="lua", version="5.1"}))
	-- "lua 5.1"  2

	print(re_gsub("$ world", "\\w+", string.upper))
	-- "$ WORLD"  1

	print(re_gsub("4+5 = $return 4+5$", "\\$(.*)\\$", function (s)
			return loadstring(s)()
		end))
	-- "4+5 = 9"  1


	-- -- gmatch
	-- i = 1
	-- for w in re.gmatch("hello world", "\\w+") do
	-- 	if i == 1 then
	-- 		print(w)  -- "hello"
	--         elseif i == 2 then
	-- 		print(w)  -- "world"
	-- 	end
	-- 	i = i + 1
	-- end
	-- print(i)  -- 3

	-- i = 1
	-- for k, v in re.gmatch("from=world, to=Lua", "(\\w+)=(\\w+)") do
	-- 	if i == 1 then
	--   		pirnt(k, v)  -- "from" "world"
	--         elseif i == 2 then
	--   		pirnt(k, v)  -- "to" "lua"
	-- 	end
	-- i = i + 1
	-- end
	-- print(i)  -- 3

	-- match ----------

	print(re_match("$$$ hello", "z"))
	-- nil

	print(re_match("$$$ hello", "\\w+"))
	-- "hello"

	print(re_match("hello world", "\\w+", 6))
	-- "world"

	print(re_match("hello world", "\\w+", -5))
	-- "world"

	print(re_match("from=world", "(\\w+)=(\\w+)"))
	-- "from" "world"
`
	l := lua.NewState()
	defer l.Close()

	RegisterRegexFuncs(l)
	if err := l.DoString(lucCode); err != nil {
		t.Error(err)
	}
}
