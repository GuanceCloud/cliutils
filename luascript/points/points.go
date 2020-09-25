package points

import (
	"encoding/json"
	"errors"
	"fmt"

	influxdb "github.com/influxdata/influxdb1-client/v2"
)

const (
	pointsCallbackFnName   = "handle"
	PointsCallbackTypeName = "points"
)

type Points struct {
	name    string
	data    []map[string]interface{}
	typelog map[string]fieldType
}

func NewPoints(name string, strongType bool, pts []*influxdb.Point) (*Points, error) {
	if name == "" {
		return nil, errors.New("name doesnot empty")
	}

	var p = Points{name: name, data: []map[string]interface{}{}}

	for _, pt := range pts {
		f, err := pt.Fields()
		if err != nil {
			return nil, err
		}

		p.data = append(p.data, map[string]interface{}{
			"name":   pt.Name(),
			"tags":   pt.Tags(),
			"fields": f,
			"time":   pt.UnixNano(),
		})
	}

	if strongType {
		p.typelog = logType(pts)
	} else {
		p.typelog = nil
	}

	return &p, nil
}

func (p *Points) Name() string {
	return p.name
}

func (p *Points) DataToLua() interface{} {
	return p.data
}

func (*Points) CallbackFnName() string {
	return pointsCallbackFnName
}

func (*Points) CallbackTypeName() string {
	return PointsCallbackTypeName
}

func (p *Points) Handle(value string, err error) {
	if err != nil {
		return
	}
	fmt.Println(value)

	m := []map[string]interface{}{}
	err = json.Unmarshal([]byte(value), &m)
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println(m)
}

// log only `int' fields
type fieldType []string

func logType(pts []*influxdb.Point) map[string]fieldType {
	fts := map[string]fieldType{}
	for _, p := range pts {
		fts[p.Name()] = filterIntFields(p)
	}
	return fts
}

func filterIntFields(pt *influxdb.Point) fieldType {
	ft := fieldType{}
	fs, err := pt.Fields()
	if err != nil {
		return nil
	}

	for k, v := range fs {
		switch v.(type) {
		case int, int8, int16, int32, int64,
			uint, uint8, uint16, uint32, uint64:
			ft = append(ft, k)
		}
	}
	return ft
}

func typeRecove(pts []*influxdb.Point, typelog map[string]fieldType) ([]*influxdb.Point, error) {
	var points []*influxdb.Point
	for _, pt := range pts {
		newpt, err := recoverIntFields(pt, typelog[pt.Name()])
		if err != nil {
			return nil, err
		}
		points = append(points, newpt)
	}
	return points, nil
}

func recoverIntFields(p *influxdb.Point, ft fieldType) (*influxdb.Point, error) {
	if len(ft) == 0 { // FIXME: need new point based on @p?
		return p, nil
	}

	fs, err := p.Fields()
	if err != nil {
		return nil, err
	}

	pn := p.Name()

	n := 0

	// NOTE: Lua do not distinguish int/float, all Golang got is float.
	// if your really need int to be float, disable type-safe in configure.
	// Loop all original int fields, they must be float now, convert to int anyway.
	// We do not check other types of fields, the Lua developer SHOULD becarefull
	// to treat them type-safe when updating exists field values, or influxdb
	// may refuse to accept the point handled by Lua.
	for _, k := range ft {
		if fs[k] == nil {
			// l.Debugf("ignore missing filed %s.%s", pn, k)
			continue
		}
		switch fs[k].(type) {
		case float32:
			fs[k] = int64(fs[k].(float32))
			n++
		case float64:
			fs[k] = int64(fs[k].(float64))
			n++
		default:
			// l.Warnf("overwrite int field(%s.%s) with conflict type: int > %v, point: %s, ft: %v",pn, k, fs[k], p.String(), ft)
		}
	}

	if n == 0 { // no field updated
		return p, nil
	} else {
		// l.Debugf("%d points type recovered", n)
		pt, err := influxdb.NewPoint(pn, p.Tags(), fs, p.Time())
		if err != nil {
			// l.Error(err)
			return nil, err
		}
		return pt, nil
	}
}
