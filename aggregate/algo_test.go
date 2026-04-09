package aggregate

import (
	T "testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestAlgoCountDistinct_Add(t *T.T) {
	t.Run("basic-types", func(t *T.T) {
		// 创建第一个计数器
		mb1 := MetricBase{
			key:      "test_field",
			name:     "test_metric",
			aggrTags: [][2]string{{"tag1", "value1"}},
		}

		calc1 := newAlgoCountDistinct(mb1, time.Now().UnixNano(), 42) // int

		// 创建第二个计数器，添加不同类型的值
		mb2 := MetricBase{
			key:      "test_field",
			name:     "test_metric",
			aggrTags: [][2]string{{"tag1", "value1"}},
		}
		calc2 := newAlgoCountDistinct(mb2, time.Now().UnixNano()+1000, 3.14) // float64

		// 创建第三个计数器，添加字符串值
		mb3 := MetricBase{
			key:      "test_field",
			name:     "test_metric",
			aggrTags: [][2]string{{"tag1", "value1"}},
		}
		calc3 := newAlgoCountDistinct(mb3, time.Now().UnixNano()+2000, "test_string") // string

		// 创建第四个计数器，添加布尔值
		mb4 := MetricBase{
			key:      "test_field",
			name:     "test_metric",
			aggrTags: [][2]string{{"tag1", "value1"}},
		}
		calc4 := newAlgoCountDistinct(mb4, time.Now().UnixNano()+3000, true) // bool

		// 合并所有计数器
		calc1.Add(calc2)
		calc1.Add(calc3)
		calc1.Add(calc4)

		// 验证不重复值的数量
		assert.Equal(t, 4, len(calc1.distinctValues))

		// 验证所有值都存在
		_, hasInt := calc1.distinctValues[42]
		_, hasFloat := calc1.distinctValues[3.14]
		_, hasString := calc1.distinctValues["test_string"]
		_, hasBool := calc1.distinctValues[true]

		assert.True(t, hasInt, "int value should exist")
		assert.True(t, hasFloat, "float value should exist")
		assert.True(t, hasString, "string value should exist")
		assert.True(t, hasBool, "bool value should exist")

		// 测试重复值不会被重复计数
		mb5 := MetricBase{
			key:      "test_field",
			name:     "test_metric",
			aggrTags: [][2]string{{"tag1", "value1"}},
		}
		calc5 := newAlgoCountDistinct(mb5, time.Now().UnixNano()+4000, 42) // 重复的int值
		calc1.Add(calc5)

		// 不重复值数量应该仍然是4
		assert.Equal(t, 4, len(calc1.distinctValues))

		// 测试Aggr方法
		points, err := calc1.Aggr()
		assert.NoError(t, err)
		assert.Len(t, points, 1)

		point := points[0]
		value, ok := point.GetI("test_field")
		assert.True(t, ok)
		assert.Equal(t, int64(4), value) // 应该是4个不重复值

		countValue, ok := point.GetI("test_field_count")
		assert.True(t, ok)
		assert.Equal(t, int64(4), countValue)

		// 验证标签
		tagValue := point.GetTag("tag1")
		assert.Equal(t, "value1", tagValue)
	})

	t.Run("nil-values", func(t *T.T) {
		// 测试nil值
		mb := MetricBase{
			key:      "test_field",
			name:     "test_metric",
			aggrTags: [][2]string{{"tag1", "value1"}},
		}

		// 注意：newAlgoCountDistinct需要传入一个值，不能是nil
		calc1 := newAlgoCountDistinct(mb, time.Now().UnixNano(), "initial")

		// 创建包含nil值的计数器（需要修改newAlgoCountDistinct以支持nil）
		// 这里我们测试其他类型的nil表示
		mb2 := MetricBase{
			key:      "test_field",
			name:     "test_metric",
			aggrTags: [][2]string{{"tag1", "value1"}},
		}
		calc2 := newAlgoCountDistinct(mb2, time.Now().UnixNano()+1000, "")

		calc1.Add(calc2)

		// 空字符串应该被视为不同的值
		assert.Equal(t, 2, len(calc1.distinctValues))
	})

	t.Run("reset-test", func(t *T.T) {
		mb := MetricBase{
			key:      "test_field",
			name:     "test_metric",
			aggrTags: [][2]string{{"tag1", "value1"}},
		}

		calc := newAlgoCountDistinct(mb, time.Now().UnixNano(), 42)

		mb2 := MetricBase{
			key:      "test_field",
			name:     "test_metric",
			aggrTags: [][2]string{{"tag1", "value1"}},
		}
		calc2 := newAlgoCountDistinct(mb2, time.Now().UnixNano()+1000, 3.14)
		calc.Add(calc2)

		assert.Equal(t, 2, len(calc.distinctValues))

		// 重置
		calc.Reset()

		assert.Equal(t, 0, len(calc.distinctValues))
		assert.Equal(t, int64(0), calc.maxTime)

		// 重置后应该可以重新添加值
		calc.Add(calc2)
		assert.Equal(t, 1, len(calc.distinctValues))
	})
}
