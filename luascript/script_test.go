package luascript

import (
	"testing"
	"time"

	influxdb "github.com/influxdata/influxdb1-client/v2"

	"gitlab.jiagouyun.com/cloudcare-tools/cliutils/luascript/points"
)

func TestScript(t *testing.T) {
	var luaCode = `
function handle(points)
	for _, pt in pairs(points) do
		for k, v in pairs(pt) do
			print("name", pt.name)
			print("time", pt.time)
			print("-------\ntags:")
			for k, v in pairs(pt.tags) do
				print(k, v)
			end
			print("-------\nfields:")
			for k, v in pairs(pt.fields) do
				print(k, v)
			end
		end
		print("-----------------------")
	end
	return points
end
`
	pt1, _ := influxdb.NewPoint("point01",
		map[string]string{
			"t1": "tags10",
			"t2": "tags20",
			"t3": "tags30",
		},
		map[string]interface{}{
			"f1": uint(11),
			"f2": int32(22),
			"f3": true,
			"f4": "hello",
		},
		time.Now(),
	)
	pt2, _ := influxdb.NewPoint("point02",
		map[string]string{
			"t1": "tags11",
			"t2": "tags21",
			"t3": "tags31",
		},
		map[string]interface{}{
			"f1": uint(33),
			"f2": int32(444),
			"f3": false,
			"f4": "world",
		},
		time.Now(),
	)

	var err error
	err = AddLuaLines("ptdata", []string{luaCode})
	if err != nil {
		t.Fatal(err)
	}

	Run()

	p, err := points.NewPoints("ptdata", false, []*influxdb.Point{pt1, pt2})
	if err != nil {
		t.Fatal(err)
	}

	Exec(p)

	time.Sleep(time.Second * 2)

	Stop()
}
