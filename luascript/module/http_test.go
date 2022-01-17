package module

import (
	"testing"

	lua "github.com/yuin/gopher-lua"
)

func TestHTTP(t *testing.T) {
	luaCode := `
	print("http test")
	response, err = http_request("GET", "http://127.0.0.1:9999", {
		-- query="",
		-- headers={
		-- 	Accept="*/*"
		-- },
		body="123123123"
	})
	if err == nil then
		print(response.body)
		print(response.status_code)
	else
		print(err)
	end
`
	l := lua.NewState()
	defer l.Close()

	RegisterHTTPFuncs(l)
	if err := l.DoString(luaCode); err != nil {
		t.Error(err)
	}
}
