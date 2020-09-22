package luascript

import (
	"sync"
	"testing"
	"time"

	influxdb "github.com/influxdata/influxdb1-client/v2"
)

func TestScript(t *testing.T) {
	var luaCode = `
function handle(points)
	for _, pt in pairs(points) do
		print("name:", pt.name)
		print("time:", pt.time)
		print("-------\ntags:")
		for k, v in pairs(pt.tags) do
			print(k, v)
		end
		print("-------\nfields:")
		for k, v in pairs(pt.fields) do
			print(k, v)
			if k == "f1" or k == "f2" then
				print("+1", v+1)
			end
		end
		print("-----------------------")
	end

	for i, pt in pairs(points) do
		pt.name = "new_name"
		pt.time = 11111111111
		for k, v in pairs(pt.tags) do
			pt.tags[k] = "new_tags_value"
		end
		for k, v in pairs(pt.fields) do
			pt.fields[k] = "new_fields_value"
		end
	end

	print("=================== modify points ==================")

	for _, pt in pairs(points) do
		print("name:", pt.name)
		print("time:", pt.time)
		print("-------\ntags:")
		for k, v in pairs(pt.tags) do
			print(k, v)
		end
		print("-------\nfields:")
		for k, v in pairs(pt.fields) do
			print(k, v)
		end
		print("-----------------------")
	end

	return points
end
print("out function")
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

	ls := NewLuaScript()
	defer ls.Stop()

	err := ls.AddLuaCode("test", []string{luaCode})
	if err != nil {
		t.Fatal(err)
	}
	ls.Run()

	wg := sync.WaitGroup{}
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			pts, err := ls.SendPoints("test", []*influxdb.Point{pt1, pt2})
			if err != nil {
				t.Log(err)
			}
			t.Logf("%v\n", pts)
		}()
	}
	wg.Wait()
}
