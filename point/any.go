// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package point

import (
	"fmt"
	"reflect"

	"google.golang.org/protobuf/proto"
	anypb "google.golang.org/protobuf/types/known/anypb"
)

func MustAnyRaw(x *anypb.Any) any {
	res, err := AnyRaw(x)
	if err != nil {
		panic(err.Error())
	}

	return res
}

// AnyRaw get original wrapped value within anypb.
func AnyRaw(x *anypb.Any) (any, error) {
	switch x.TypeUrl {
	case "type.googleapis.com/point.Array":
		var arr Array
		if err := proto.Unmarshal(x.Value, &arr); err != nil {
			return nil, err
		}

		var res []any
		for _, v := range arr.Arr {
			switch v.GetX().(type) {
			case *BasicTypes_I:
				res = append(res, v.GetI())
			case *BasicTypes_U:
				res = append(res, v.GetU())
			case *BasicTypes_F:
				res = append(res, v.GetF())
			case *BasicTypes_B:
				res = append(res, v.GetB())
			case *BasicTypes_D:
				res = append(res, v.GetD())
			case *BasicTypes_S:
				res = append(res, v.GetS())
			default: // pass
				return nil, fmt.Errorf("unknown type %q within array", reflect.TypeOf(v.GetX()).String())
			}
		}

		return res, nil

	case "type.googleapis.com/point.Map":
		var m Map
		if err := proto.Unmarshal(x.Value, &m); err != nil {
			return nil, err
		}

		res := map[string]any{}
		for k, v := range m.Map {
			switch v.GetX().(type) {
			case *BasicTypes_I:
				res[k] = v.GetI()
			case *BasicTypes_U:
				res[k] = v.GetU()
			case *BasicTypes_F:
				res[k] = v.GetF()
			case *BasicTypes_B:
				res[k] = v.GetB()
			case *BasicTypes_D:
				res[k] = v.GetD()
			case *BasicTypes_S:
				res[k] = v.GetS()
			default:
				return nil, fmt.Errorf("unknown type %q within map", reflect.TypeOf(v.GetX()).String())
			}
		}

		return res, nil

	default:
		return nil, fmt.Errorf("unknown type %q", x.TypeUrl)
	}
}

func MustNewArray(ents []any) *Array {
	x, err := NewArray(ents)
	if err != nil {
		panic(err.Error())
	}
	return x
}

// NewArray create array value that can be used in point field.
// The types within ents can be mixed basic types.
func NewArray(ents []any) (arr *Array, err error) {
	arr = &Array{
		Arr: make([]*BasicTypes, 0, len(ents)),
	}

	for _, v := range ents {
		switch x := v.(type) {
		case int8:
			arr.Arr = append(arr.Arr, &BasicTypes{X: &BasicTypes_I{int64(x)}})
		case uint8:
			arr.Arr = append(arr.Arr, &BasicTypes{X: &BasicTypes_U{uint64(x)}})
		case int16:
			arr.Arr = append(arr.Arr, &BasicTypes{X: &BasicTypes_I{int64(x)}})
		case uint16:
			arr.Arr = append(arr.Arr, &BasicTypes{X: &BasicTypes_U{uint64(x)}})
		case int32:
			arr.Arr = append(arr.Arr, &BasicTypes{X: &BasicTypes_I{int64(x)}})
		case uint32:
			arr.Arr = append(arr.Arr, &BasicTypes{X: &BasicTypes_U{uint64(x)}})
		case int:
			arr.Arr = append(arr.Arr, &BasicTypes{X: &BasicTypes_I{int64(x)}})
		case uint:
			arr.Arr = append(arr.Arr, &BasicTypes{X: &BasicTypes_U{uint64(x)}})
		case int64:
			arr.Arr = append(arr.Arr, &BasicTypes{X: &BasicTypes_I{x}})
		case uint64:
			arr.Arr = append(arr.Arr, &BasicTypes{X: &BasicTypes_U{x}})
		case float64:
			arr.Arr = append(arr.Arr, &BasicTypes{X: &BasicTypes_F{x}})
		case float32:
			arr.Arr = append(arr.Arr, &BasicTypes{X: &BasicTypes_F{float64(x)}})
		case string:
			arr.Arr = append(arr.Arr, &BasicTypes{X: &BasicTypes_S{x}})
		case []byte:
			arr.Arr = append(arr.Arr, &BasicTypes{X: &BasicTypes_D{x}})
		case bool:
			arr.Arr = append(arr.Arr, &BasicTypes{X: &BasicTypes_B{x}})
		case nil:
			arr.Arr = append(arr.Arr, nil)
		default:
			return nil, fmt.Errorf("unknown type %q within array", reflect.TypeOf(v).String())
		}
	}

	return nil, nil
}

func MustNewMap(ents map[string]any) *Map {
	x, err := NewMap(ents)
	if err != nil {
		panic(err.Error())
	}

	return x
}

// NewMap create map value that can be used in point field.
func NewMap(ents map[string]any) (dict *Map, err error) {
	dict = &Map{
		Map: map[string]*BasicTypes{},
	}

	for k, v := range ents {
		switch x := v.(type) {
		case int8:
			dict.Map[k] = &BasicTypes{X: &BasicTypes_I{int64(x)}}
		case uint8:
			dict.Map[k] = &BasicTypes{X: &BasicTypes_U{uint64(x)}}
		case int16:
			dict.Map[k] = &BasicTypes{X: &BasicTypes_I{int64(x)}}
		case uint16:
			dict.Map[k] = &BasicTypes{X: &BasicTypes_U{uint64(x)}}
		case int32:
			dict.Map[k] = &BasicTypes{X: &BasicTypes_I{int64(x)}}
		case uint32:
			dict.Map[k] = &BasicTypes{X: &BasicTypes_U{uint64(x)}}
		case int:
			dict.Map[k] = &BasicTypes{X: &BasicTypes_I{int64(x)}}
		case uint:
			dict.Map[k] = &BasicTypes{X: &BasicTypes_U{uint64(x)}}
		case int64:
			dict.Map[k] = &BasicTypes{X: &BasicTypes_I{x}}
		case uint64:
			dict.Map[k] = &BasicTypes{X: &BasicTypes_U{x}}
		case float64:
			dict.Map[k] = &BasicTypes{X: &BasicTypes_F{x}}
		case float32:
			dict.Map[k] = &BasicTypes{X: &BasicTypes_F{float64(x)}}
		case string:
			dict.Map[k] = &BasicTypes{X: &BasicTypes_S{x}}
		case []byte:
			dict.Map[k] = &BasicTypes{X: &BasicTypes_D{x}}
		case bool:
			dict.Map[k] = &BasicTypes{X: &BasicTypes_B{x}}
		case nil:
			dict.Map[k] = nil
		default: // value ignored
			return nil, fmt.Errorf("unknown type %q within map", reflect.TypeOf(v).String())
		}
	}

	return nil, nil
}

// NewAny create anypb based on exist proto message.
func NewAny(x proto.Message) (*anypb.Any, error) {
	return anypb.New(x)
}

func MustNewAny(x proto.Message) *anypb.Any {
	if a, err := anypb.New(x); err != nil {
		panic(err.Error())
	} else {
		return a
	}
}
