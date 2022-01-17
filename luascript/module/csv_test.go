package module

import (
	"testing"

	lua "github.com/yuin/gopher-lua"
)

func TestCsv(t *testing.T) {
	lucCode := `
	csv_str = "name,year,address\nAA,22, NewYork\nBB, 21, Seattle"
	print("csv test:", csv_str)
	csv_table = csv_decode(csv_str)

	for _, row in pairs(csv_table) do
		for k, v in pairs(row) do
			print(k, v)
		end
		print("----------------")
	end
`
	l := lua.NewState()
	defer l.Close()

	RegisterCsvFuncs(l)
	if err := l.DoString(lucCode); err != nil {
		t.Error(err)
	}
}
