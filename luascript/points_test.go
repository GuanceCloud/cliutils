package module

import (
	"reflect"
	"testing"
	"time"

	influxdb "github.com/influxdata/influxdb1-client/v2"
	lua "github.com/yuin/gopher-lua"
)

func TestSendMetatable(t *testing.T) {

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
end`

	l := lua.NewState()
	defer l.Close()

	if err := l.DoString(luaCode); err != nil {
		t.Fatal(err)
	}

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

	ret, err := sendMetatable(l, []*influxdb.Point{pt1, pt2})
	if err != nil {
		t.Fatal(err)
	}

	t.Log("type: ", reflect.TypeOf(ret))
	t.Log("table: ", *ret)

	if pts, err := table2Points(ret); err != nil {
		t.Fatal(err)
	} else {
		t.Log("points: ", pts)
	}
}

func TestAddPoint(t *testing.T) {

	var luaCode = `
function handle(points)
	table.insert(points,
		{
			name="create_name1",
			time=222222222,
			tags={ t1="create_tags_01", t2="create_tags_02" },
			fields={ f1="create_fields_01", f5=555555, f6=true }
		}
	)

	print("=================== insert points ==================")

	for _, pt in pairs(points) do
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
		print("-----------------------")
	end

	return points
end`

	l := lua.NewState()
	defer l.Close()

	if err := l.DoString(luaCode); err != nil {
		t.Fatal(err)
	}

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

	ret, err := sendMetatable(l, []*influxdb.Point{pt1})
	if err != nil {
		t.Fatal(err)
	}

	if pts, err := table2Points(ret); err != nil {
		t.Fatal(err)
	} else {
		t.Log("points: ", pts)
	}
}
