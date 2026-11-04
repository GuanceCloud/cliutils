// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package runner

import "encoding/json"

// SyntheticsContext is the correlation contract consumed by an existing Web RUM
// SDK. It contains public identifiers only, never task credentials or tokens.
type SyntheticsContext struct {
	TestID   string `json:"test_id"`
	ResultID string `json:"result_id"`
}

// InitScript must run in every new document before page scripts initialize RUM.
// A standalone runner without a platform task ID leaves RUM behavior unchanged.
func (c SyntheticsContext) InitScript() (string, error) {
	if c.TestID == "" || c.ResultID == "" {
		return "", nil
	}
	data, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	return `(function () {
const context = ` + string(data) + `;
window._DATAFLUX_SYNTHETICS_BROWSER = true;
window._GUANCE_SYNTHETICS_PUBLIC_ID = context.test_id;
window._GUANCE_SYNTHETICS_RESULT_ID = context.result_id;
})();`, nil
}
