package module

import (
	"testing"

	lua "github.com/yuin/gopher-lua"
)

func TestXml(t *testing.T) {
	lucCode := `
        xml_str ="<booklist><book>100</book><book>100.5</book><book>200</book></booklist>"
	print("xml test:", xml_str)
	xml_table = xml_decode(xml_str)

	for _, row in pairs(xml_table) do
		for k, v in pairs(row) do
			for _, vv in pairs(v) do
				print(vv)
				print("number add 1", vv+1)
			end
		end
		print("----------------")
	end
`
	l := lua.NewState()
	defer l.Close()

	RegisterXmlFuncs(l)
	if err := l.DoString(lucCode); err != nil {
		t.Error(err)
	}
}
