package cliutils

import (
	"fmt"
	"time"

	"github.com/influxdata/influxdb1-client/models"
	influxdb "github.com/influxdata/influxdb1-client/v2"
)

func ParseLineProto(data []byte, precision string) error {
	if data == nil || len(data) == 0 {
		return fmt.Errorf("empty data")
	}

	_, err := models.ParsePointsWithPrecision(data, time.Now().UTC(), precision)
	return err
}

type Option struct {
	Time      time.Time
	Precision string
	ExtraTags map[string]string
}

func ParsePoints(data []byte, opt *Option) ([]*influxdb.Point, error) {

	if len(data) == 0 {
		return nil, fmt.Errorf("empty data")
	}

	if opt == nil {
		opt = &Option{Precision: "n"}
	}

	points, err := models.ParsePointsWithPrecision(data, opt.Time, opt.Precision)
	if err != nil {
		return nil, err
	}

	res := []*influxdb.Point{}
	for _, point := range points {

		if opt.ExtraTags != nil {
			for k, v := range opt.ExtraTags {
				if !point.HasTag([]byte(k)) {
					point.AddTag(k, v)
				}
			}
		}

		res = append(res, influxdb.NewPointFrom(point))
	}

	return res, nil
}

func MakeLineProtoPoint(name string,
	tags map[string]string,
	fields map[string]interface{},
	opt *Option,
	t ...time.Time) ([]byte, error) {

	return nil, nil
}
