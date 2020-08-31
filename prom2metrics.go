package cliutils

import (
	"fmt"
	"io"
	"strings"
	"time"

	ifxcli "github.com/influxdata/influxdb1-client/v2"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
)

/// prometheus 数据转行协议 point
///
/// prometheus 数据以 K/V 描述，以 `go_gc_duration_seconds{quantile="0"} 7.4545e-05` 为例
///
/// 转换规则
///     1. measurement： 取 K 的第一个下划线，左右临近字符串。示例 measurement 为 `go_gc`
///
///     2. 所有大括号中的数据，全部做成 Key/Value 形式的 tags
///
///     3. 允许手动添加字符串前缀，如果前缀为空字符串，则不添加。例如 measurementPrefix 为 `cloudcare`，measurement 为 `cloudcare_go_gc`
///
///     4. 允许设置默认 measurement，当 point 没有 tags 时，使用默认 measurement。默认 measurement 不允许为空字符串。
///
///     5. 允许设置默认时间，当无法解析 prometheus 数据的 timestamp 时，使用默认时间
///
///     6. 如果遇到空数据，则跳过执行下一条
///
///  具体输出，参照测试用例 prom2metrics_test.go

type parse struct {
	metricName         string
	measurement        string
	defaultMeasurement string
	metrics            []*dto.Metric
	t                  time.Time
}

func PromTextToMetrics(data io.Reader, measurementPrefix, defaultMeasurement string, t time.Time) ([]*ifxcli.Point, error) {
	if defaultMeasurement == "" {
		return nil, fmt.Errorf("invalid defaultMeasurement, it is empty")
	}

	var parser expfmt.TextParser
	metrics, err := parser.TextToMetricFamilies(data)
	if err != nil {
		return nil, err
	}

	var pts []*ifxcli.Point
	for name, metric := range metrics {
		measurement := name
		if measurementPrefix != "" {
			measurement = getMeasurement(name, measurementPrefix)
		}

		p := parse{
			metricName:         name,
			measurement:        measurement,
			defaultMeasurement: defaultMeasurement,
			metrics:            metric.GetMetric(),
			t:                  t,
		}

		switch metric.GetType() {
		case dto.MetricType_COUNTER:
			pts = append(pts, p.counter()...)

		case dto.MetricType_GAUGE:
			pts = append(pts, p.gauge()...)

		case dto.MetricType_SUMMARY:
			pts = append(pts, p.summary()...)

		case dto.MetricType_UNTYPED:
			pts = append(pts, p.untyped()...)

		case dto.MetricType_HISTOGRAM:
			pts = append(pts, p.histogram()...)

		}
	}
	return pts, nil
}

func (p *parse) counter() []*ifxcli.Point {
	var pts []*ifxcli.Point

	for _, m := range p.metrics {
		counter := m.GetCounter()
		if counter == nil {
			continue
		}

		tags := labelToTags(m.GetLabel())
		fields := map[string]interface{}{p.metricName: counter.GetValue()}

		pt, err := p.newPoint(tags, fields, m.GetTimestampMs())
		if err != nil {
			continue
		}
		pts = append(pts, pt)
	}
	return pts
}

func (p *parse) gauge() []*ifxcli.Point {
	var pts []*ifxcli.Point

	for _, m := range p.metrics {
		gauge := m.GetGauge()
		if gauge == nil {
			continue
		}

		tags := labelToTags(m.GetLabel())
		fields := map[string]interface{}{p.metricName: gauge.GetValue()}

		pt, err := p.newPoint(tags, fields, m.GetTimestampMs())
		if err != nil {
			continue
		}
		pts = append(pts, pt)
	}
	return pts
}

func (p *parse) summary() []*ifxcli.Point {
	var pts []*ifxcli.Point

	for _, m := range p.metrics {
		fmt.Println(m)
		summary := m.GetSummary()
		if summary == nil {
			continue
		}

		for _, quantile := range summary.GetQuantile() {
			tags := map[string]string{"quantile": fmt.Sprintf("%.3f", quantile.GetQuantile())}
			fields := map[string]interface{}{p.metricName: quantile.GetValue()}

			pt, err := p.newPoint(tags, fields, m.GetTimestampMs())
			if err != nil {
				continue
			}
			pts = append(pts, pt)
		}

		tags := labelToTags(m.GetLabel())
		fields := map[string]interface{}{
			p.metricName + "_count": int64(summary.GetSampleCount()),
			p.metricName + "_sum":   summary.GetSampleSum(),
		}

		pt, err := p.newPoint(tags, fields, m.GetTimestampMs())
		if err != nil {
			continue
		}
		pts = append(pts, pt)
	}
	return pts
}

func (p *parse) untyped() []*ifxcli.Point {
	var pts []*ifxcli.Point

	for _, m := range p.metrics {
		untyped := m.GetUntyped()
		if untyped == nil {
			continue
		}
		fields := map[string]interface{}{p.metricName: untyped.GetValue()}

		pt, err := p.newPoint(nil, fields, m.GetTimestampMs())
		if err != nil {
			continue
		}
		pts = append(pts, pt)
	}
	return pts
}

func (p *parse) histogram() []*ifxcli.Point {
	var pts []*ifxcli.Point

	for _, m := range p.metrics {
		histogram := m.GetHistogram()
		if histogram == nil {
			continue
		}

		tags := labelToTags(m.GetLabel())
		fields := map[string]interface{}{
			p.metricName + "_count": int64(histogram.GetSampleCount()),
			p.metricName + "_sum":   histogram.GetSampleSum(),
		}

		pt, err := p.newPoint(tags, fields, m.GetTimestampMs())
		if err != nil {
			continue
		}
		pts = append(pts, pt)

		for _, bucket := range histogram.GetBucket() {
			tags["le"] = fmt.Sprintf("%.3f", bucket.GetUpperBound())
			fields := map[string]interface{}{p.metricName + "_bucket": int64(bucket.GetCumulativeCount())}

			pt, err := p.newPoint(tags, fields, m.GetTimestampMs())
			if err != nil {
				continue
			}
			pts = append(pts, pt)
		}
	}
	return pts
}

func (p *parse) newPoint(tags map[string]string, fields map[string]interface{}, ts int64) (*ifxcli.Point, error) {
	if ts > 0 {
		p.t = time.Unix(0, ts*int64(time.Millisecond))
	}
	var measurement string
	if tags == nil {
		measurement = p.defaultMeasurement
	} else {
		measurement = p.measurement
	}
	return ifxcli.NewPoint(measurement, tags, fields, p.t)
}

func getMeasurement(name, measurementPrefix string) string {
	nameBlocks := strings.Split(name, "_")
	if len(nameBlocks) > 2 {
		name = strings.Join(nameBlocks[:2], "_")
	}
	if !strings.HasPrefix(name, measurementPrefix) {
		name = measurementPrefix + "_" + name
	}
	return name
}

func labelToTags(label []*dto.LabelPair) map[string]string {
	if len(label) == 0 {
		return nil
	}
	var tags = make(map[string]string, len(label))
	for _, lab := range label {
		tags[lab.GetName()] = lab.GetValue()
	}
	return tags
}
