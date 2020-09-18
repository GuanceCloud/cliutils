package module

import (
	"testing"
	"time"

	influxdb "github.com/influxdata/influxdb1-client/v2"
)

func TestLMode(t *testing.T) {

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

	l := NewLuaMode()
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

	pts, err := l.PointsOnHandle([]*influxdb.Point{pt1})
	if err != nil {
		t.Fatal(err)
	}

	t.Log("points: ", pts)
}
