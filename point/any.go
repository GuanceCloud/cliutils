// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package point

// NewArray create array value that can be used in point field.
// The types within ents can be mixed basic types
func NewArray(ents []any) (arr *Array) {
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
			arr.Arr = append(arr.Arr, &BasicTypes{X: &BasicTypes_I{int64(x)}})
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
		default: // value ignored
		}
	}

	return
}

// NewMap create map value that can be used in point field.
func NewMap(ents map[string]any) (dict *Map) {
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
			dict.Map[k] = &BasicTypes{X: &BasicTypes_I{int64(x)}}
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
		default: // value ignored
		}
	}

	return
}
