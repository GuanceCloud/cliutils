package module

import (
	"testing"

	lua "github.com/yuin/gopher-lua"
)

func TestJson(t *testing.T) {
	lucCode := `
	json_str = '{ "hostname":"ubuntu18.04LTS", "date":"2019年12月10日 星期二 11时14分47秒 CST", "ip":["127.0.0.1","192.168.0.1","172.16.0.1"] }'

	print("json test: ", json_str)
	json_table = json_decode(json_str)

	for k, v in pairs(json_table) do
		print(k, v)
	end
	for _, v in pairs(json_table["ip"]) do
		print(v)
	end
`
	l := lua.NewState()
	defer l.Close()

	RegisterJsonFuncs(l)
	if err := l.DoString(lucCode); err != nil {
		t.Error(err)
	}
}
