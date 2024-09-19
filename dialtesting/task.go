// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

// Package dialtesting defined dialtesting tasks and task implements.
package dialtesting

import (
	"fmt"
	"net/url"
	"time"
)

const (
	StatusStop = "stop"

	ClassHTTP      = "HTTP"
	ClassTCP       = "TCP"
	ClassWebsocket = "WEBSOCKET"
	ClassICMP      = "ICMP"
	ClassDNS       = "DNS"
	ClassHeadless  = "BROWSER"

	ClassOther = "OTHER"
)

type Task interface {
	ID() string
	Status() string
	Run() error
	Init() error
	InitDebug() error
	CheckResult() ([]string, bool)
	Class() string
	GetResults() (map[string]string, map[string]interface{})
	PostURLStr() string
	MetricName() string
	Stop() error
	RegionName() string
	AccessKey() string
	Check() error
	UpdateTimeUs() int64
	GetFrequency() string
	GetOwnerExternalID() string
	SetOwnerExternalID(string)
	GetLineData() string
	GetHostName() (string, error)
	GetWorkspaceLanguage() string
	GetDFLabel() string

	SetRegionID(string)
	SetAk(string)
	SetStatus(string)
	SetUpdateTime(int64)

	Ticker() *time.Ticker
}

func getHostName(host string) (string, error) {
	reqURL, err := url.Parse(host)
	if err != nil {
		return "", fmt.Errorf("parse host error: %w", err)
	}

	return reqURL.Hostname(), nil
}
