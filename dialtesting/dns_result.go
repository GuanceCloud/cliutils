// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package dialtesting

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type dnsRecord struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	Class string `json:"class"`
	TTL   uint32 `json:"ttl"`
	Value string `json:"value"`
}

type dnsAttempt struct {
	Transport string `json:"transport"`
	Cost      int64  `json:"response_time"`
	Truncated bool   `json:"truncated"`
}

type dnsQueryResult struct {
	Type         string          `json:"query_type"`
	Name         string          `json:"query_name,omitempty"`
	Server       string          `json:"server,omitempty"`
	Port         string          `json:"port,omitempty"`
	Status       string          `json:"status"`
	RCode        string          `json:"rcode,omitempty"`
	Cost         *int64          `json:"response_time,omitempty"`
	Count        int             `json:"answer_count"`
	TargetCount  int             `json:"target_record_count"`
	Records      []dnsRecord     `json:"records"`
	Attempts     []dnsAttempt    `json:"attempts,omitempty"`
	Flags        map[string]bool `json:"flags,omitempty"`
	RequestFlags map[string]bool `json:"request_flags,omitempty"`
	Code         string          `json:"error_code,omitempty"`
	Error        string          `json:"error_message,omitempty"`
	Truncated    bool            `json:"truncated,omitempty"`
}

type dnsRunResult struct {
	BootstrapSource                           string
	Queries                                   []dnsQueryResult
	Cost, Bootstrap, Handshake, QueryCost     time.Duration
	Started, BootstrapStarted, QueryStarted   bool
	IP, Code, Error, TLSVersion, HTTPProtocol string
	Verified                                  *bool
	CertNotAfter                              int64
	HTTPStatus                                int
}

type dnsAssertionResult struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	RecordType string `json:"record_type,omitempty"`
	Operator   string `json:"operator,omitempty"`
	Expected   any    `json:"expected"`
	Actual     any    `json:"actual,omitempty"`
	Unit       string `json:"unit,omitempty"`
	Status     string `json:"status"`
	Reason     string `json:"reason,omitempty"`
	Truncated  bool   `json:"actual_truncated,omitempty"`
}

func (t *DNSTask) prepareAssertions(allowTemplate bool) error {
	templated := func(s string) bool { return allowTemplate && hasTemplateTags(s) }
	count := 0
	needed := map[string]bool{}
	for _, rule := range t.SuccessWhen {
		if rule == nil {
			return errors.New("DNS success rule must not be null")
		}
		before := count
		if rule.ResponseTime != "" {
			d, err := time.ParseDuration(rule.ResponseTime)
			if !templated(rule.ResponseTime) && (err != nil || d <= 0) {
				return errors.New("invalid DNS response_time threshold")
			}
			count++
		}
		if rule.AuthenticatedData != nil {
			count++
		}
		for _, scope := range []*DNSRecordAssertions{rule.AtLeastOneRecord, rule.EveryRecord} {
			if scope == nil {
				continue
			}
			n := 0
			for typ, conditions := range scope.conditions() {
				for _, c := range conditions {
					if err := validateDNSCondition(c, allowTemplate); err != nil {
						return err
					}
					needed[typ] = true
					count++
					n++
				}
			}
			if n == 0 {
				return errors.New("DNS record condition must not be empty")
			}
		}
		if before == count {
			return errors.New("empty DNS success rule")
		}
	}
	if count < 1 || count > 20 {
		return errors.New("DNS requires 1 to 20 conditions")
	}
	selected := map[string]bool{}
	deferredTypes := false
	for _, typ := range t.RecordTypes {
		if templated(typ) {
			deferredTypes = true
			continue
		}
		typ = strings.ToUpper(typ)
		valid := false
		for _, allowed := range dnsRecordTypes {
			if typ == allowed {
				valid = true
			}
		}
		if !valid {
			return errors.New("unsupported DNS record type")
		}
		selected[typ] = true
	}
	if len(t.RecordTypes) == 0 {
		selected = needed
		if len(selected) == 0 {
			selected["A"] = true
		}
	} else {
		for typ := range needed {
			if !selected[typ] && !deferredTypes {
				return fmt.Errorf("record type %s is required by an assertion", typ)
			}
		}
	}
	t.queryTypes = nil
	for _, typ := range dnsRecordTypes {
		if selected[typ] {
			t.queryTypes = append(t.queryTypes, typ)
		}
	}
	return nil
}

func validateDNSCondition(c *DNSRecordCondition, allowTemplate bool) error {
	if c == nil {
		return errors.New("null DNS record condition")
	}
	n := 0
	if c.Exists {
		n++
	}
	for _, s := range []string{c.Is, c.Contains, c.MatchRegex, c.NotMatchRegex} {
		if s != "" {
			n++
		}
	}
	if n != 1 || c.IsNot != "" || c.NotContains != "" {
		return errors.New("DNS record condition requires exactly one supported comparator")
	}
	if len(c.MatchRegex) > 4096 || len(c.NotMatchRegex) > 4096 {
		return errors.New("DNS regular expression is too long")
	}
	c.matchRe, c.notMatchRe = nil, nil
	if allowTemplate && (hasTemplateTags(c.MatchRegex) || hasTemplateTags(c.NotMatchRegex)) {
		return nil
	}
	if err := genReg(&c.SuccessOption); err != nil {
		return errors.New("invalid DNS record regular expression")
	}
	return nil
}

func dnsEqual(typ, a, b string) bool {
	switch typ {
	case "A", "AAAA":
		x, e1 := netip.ParseAddr(a)
		y, e2 := netip.ParseAddr(b)
		return e1 == nil && e2 == nil && x.Unmap() == y.Unmap()
	case "CNAME", "NS":
		return strings.EqualFold(strings.TrimSuffix(a, "."), strings.TrimSuffix(b, "."))
	case "MX":
		x, y := strings.Fields(a), strings.Fields(b)
		if len(x) != 2 || len(y) != 2 {
			return false
		}
		xp, xe := strconv.ParseUint(x[0], 10, 16)
		yp, ye := strconv.ParseUint(y[0], 10, 16)
		return xe == nil && ye == nil && xp == yp && dnsEqual("NS", x[1], y[1])
	default:
		return a == b
	}
}

func dnsRecordMatches(typ string, c *DNSRecordCondition, value string) bool {
	if c.Exists {
		return true
	}
	if c.Is != "" {
		return dnsEqual(typ, value, c.Is)
	}
	return c.SuccessOption.check(value, "DNS record") == nil
}

func dnsConditionOperator(c *DNSRecordCondition) (string, any) {
	switch {
	case c.Exists:
		return "exists", true
	case c.Is != "":
		return "is", c.Is
	case c.Contains != "":
		return "contains", c.Contains
	case c.MatchRegex != "":
		return "match_regex", c.MatchRegex
	default:
		return "not_match_regex", c.NotMatchRegex
	}
}

func (t *DNSTask) assertionResults() []dnsAssertionResult {
	var rows []dnsAssertionResult
	baseOK := t.result.Error == "" && len(t.result.Queries) > 0
	for i, rule := range t.SuccessWhen {
		if rule == nil {
			continue
		}
		prefix := fmt.Sprintf("success_when[%d]", i)
		if rule.ResponseTime != "" {
			d, _ := time.ParseDuration(rule.ResponseTime)
			r := dnsAssertionResult{ID: prefix + ".response_time", Type: "response_time", Expected: rule.ResponseTime, Actual: t.result.Cost.Microseconds(), Unit: "us", Status: "passed"}
			if t.result.Cost >= d {
				r.Status, r.Reason = "failed", "total response time is not below threshold"
			}
			rows = append(rows, r)
		}
		if rule.AuthenticatedData != nil {
			actual := map[string]bool{}
			pass := true
			for _, q := range t.result.Queries {
				actual[q.Type] = q.Flags["AD"]
				if q.Flags["AD"] != *rule.AuthenticatedData {
					pass = false
				}
			}
			r := dnsAssertionResult{ID: prefix + ".authenticated_data", Type: "authenticated_data", Expected: *rule.AuthenticatedData, Actual: actual, Status: "passed"}
			if !pass {
				r.Status, r.Reason = "failed", "resolver AD declaration does not match"
			}
			rows = append(rows, r)
		}
		for _, scope := range []struct {
			name  string
			every bool
			rules *DNSRecordAssertions
		}{{"at_least_one_record", false, rule.AtLeastOneRecord}, {"every_record", true, rule.EveryRecord}} {
			for _, typ := range dnsRecordTypes {
				for index, c := range scope.rules.conditions()[typ] {
					if c == nil {
						continue
					}
					values := []string{}
					for _, q := range t.result.Queries {
						if q.Type == typ {
							for _, rec := range q.Records {
								if rec.Type == typ {
									values = append(values, rec.Value)
								}
							}
						}
					}
					pass := len(values) > 0
					matched := 0
					for _, v := range values {
						if dnsRecordMatches(typ, c, v) {
							matched++
						}
					}
					if scope.every {
						pass = pass && matched == len(values)
					} else {
						pass = pass && matched > 0
					}
					op, expected := dnsConditionOperator(c)
					kind := "record_some"
					if scope.every {
						kind = "record_every"
					}
					r := dnsAssertionResult{ID: fmt.Sprintf("%s.%s.of_type_%s[%d]", prefix, scope.name, strings.ToLower(typ), index), Type: kind, RecordType: typ, Operator: op, Expected: expected, Actual: values, Status: "passed"}
					if !pass {
						r.Status, r.Reason = "failed", "record condition was not satisfied"
					}
					rows = append(rows, r)
				}
			}
		}
	}
	if !baseOK {
		for i := range rows {
			rows[i].Status, rows[i].Reason, rows[i].Actual = "skipped", "no complete successful DNS execution", nil
		}
	}
	return rows
}

func (t *DNSTask) checkResult() ([]string, bool) {
	if t.result.Error != "" {
		return []string{t.safeDNSString(t.result.Error)}, false
	}
	reasons := []string{}
	anyPassed := false
	for _, r := range t.assertionResults() {
		if r.Status == "passed" {
			anyPassed = true
		} else {
			reasons = append(reasons, r.ID+": "+r.Reason)
		}
	}
	return reasons, anyPassed
}

func dnsJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "null"
	}
	return string(b)
}

func (t *DNSTask) getResults() (map[string]string, map[string]any) {
	reasons, anyPassed := t.checkResult()
	ok := t.result.Error == "" && len(reasons) == 0 && anyPassed
	if t.SuccessWhenLogic == "or" {
		ok = t.result.Error == "" && anyPassed
	}
	tags := map[string]string{"name": t.Name, "domain": t.Domain, "dns_server": t.host, "dest_host": t.host, "dest_port": t.Port, "proto": "dns", "dns_protocol": t.Protocol, "status": "FAIL"}
	if t.result.IP != "" {
		tags["dest_ip"] = t.result.IP
	}
	for k, v := range t.Tags {
		if _, reserved := tags[k]; !reserved && k != "dest_ip" {
			tags[k] = v
		}
	}
	for key, value := range tags {
		switch key {
		case "status", "proto", "dns_protocol": // Fixed enums contain no user values.
		default:
			tags[key] = t.safeDNSString(value)
		}
	}
	fields := map[string]any{"success": int64(-1), "query_count": int64(len(t.queryTypes)), "completed_query_count": int64(0), "answer_count": int64(0)}
	if t.result.Started {
		fields["response_time"] = t.result.Cost.Microseconds()
		if t.result.BootstrapStarted {
			fields["bootstrap_time"] = t.result.Bootstrap.Microseconds()
		}
		if t.result.QueryStarted {
			fields["dns_query_time"] = t.result.QueryCost.Microseconds()
		}
	}
	if ok {
		tags["status"] = "OK"
		fields["success"] = int64(1)
	} else {
		code := t.result.Code
		if code == "" {
			code = "assertion_failed"
		}
		fields["error_code"], fields["fail_reason"] = code, t.safeDNSString(strings.Join(reasons, "; "))
		if fields["fail_reason"] == "" {
			fields["fail_reason"] = "DNS execution did not produce a successful result"
		}
	}
	if t.result.BootstrapSource != "" {
		fields["bootstrap_source"] = t.result.BootstrapSource
	}
	queries := append([]dnsQueryResult{}, t.result.Queries...)
	for i := range queries {
		q := &queries[i]
		if q.RCode != "" {
			fields["completed_query_count"] = fields["completed_query_count"].(int64) + 1
		}
		fields["answer_count"] = fields["answer_count"].(int64) + int64(q.Count)
		q.Records = append([]dnsRecord{}, q.Records...)
		if len(q.Records) > 256 {
			q.Records = q.Records[:256]
			q.Truncated = true
		}
		for j := range q.Records {
			q.Records[j].Value = t.safeDNSString(q.Records[j].Value)
			q.Records[j].Name = t.safeDNSString(q.Records[j].Name)
		}
		q.Error = t.safeDNSString(q.Error)
		q.Name, q.Server = t.safeDNSString(q.Name), t.safeDNSString(q.Server)
	}
	rows := t.assertionResults()
	for i := range rows {
		rows[i].Expected = t.sanitizeDNSValue(rows[i].Expected)
		rows[i].Actual = t.sanitizeDNSValue(rows[i].Actual)
	}
	budget := max(128, min(MaxMsgSize/2, 32768))
	trimmed := false
	for len(dnsJSON(queries)) > budget {
		changed := false
		for i := range queries {
			if len(queries[i].Records) > 0 {
				queries[i].Records = queries[i].Records[:len(queries[i].Records)/2]
				queries[i].Truncated = true
				changed = true
			}
		}
		trimmed = true
		if !changed {
			queries = []dnsQueryResult{}
			break
		}
	}
	for i := range queries {
		trimmed = trimmed || queries[i].Truncated
	}
	if len(dnsJSON(rows)) > budget {
		for i := range rows {
			rows[i].Actual = nil
			rows[i].Truncated = true
		}
		trimmed = true
	}
	if len(dnsJSON(rows)) > budget {
		rows = []dnsAssertionResult{}
	}
	fields["dns_queries"], fields["assertion_results"], fields["details_truncated"] = dnsJSON(queries), dnsJSON(rows), trimmed
	if t.result.Verified != nil {
		fields["certificate_verified"] = *t.result.Verified
	}
	if t.result.TLSVersion != "" {
		fields["tls_version"] = t.result.TLSVersion
		fields["server_name"] = t.safeDNSString(t.tlsServerName())
		fields["handshake_time"] = t.result.Handshake.Microseconds()
	}
	if t.result.CertNotAfter != 0 {
		fields["ssl_cert_not_after"] = t.result.CertNotAfter
	}
	if t.result.HTTPProtocol != "" {
		fields["http_protocol"] = t.result.HTTPProtocol
		fields["http_status_code"] = int64(t.result.HTTPStatus)
	}
	fields["message"] = dnsJSON(map[string]any{"status": tags["status"], "error_code": fields["error_code"], "fail_reason": fields["fail_reason"]})
	return tags, fields
}

func (t *DNSTask) safeDNSString(s string) string {
	if t.DoH != nil {
		for _, value := range t.DoH.Headers {
			if value != "" {
				s = strings.ReplaceAll(s, value, "[redacted]")
			}
		}
		if u, err := url.Parse(t.DoH.URL); err == nil {
			for _, values := range u.Query() {
				for _, value := range values {
					if value != "" {
						s = strings.ReplaceAll(s, url.QueryEscape(value), "[redacted]")
						s = strings.ReplaceAll(s, value, "[redacted]")
					}
				}
			}
		}
	}
	for _, v := range t.ConfigVars {
		if v != nil && v.Secure && v.Value != "" {
			s = strings.ReplaceAll(s, v.Value, "[redacted]")
		}
	}
	if t.Certificate != nil && t.Certificate.PrivateKey != "" {
		s = strings.ReplaceAll(s, t.Certificate.PrivateKey, "[redacted]")
	}
	if len(s) > 4096 {
		s = strings.ToValidUTF8(s[:4096], "")
	}
	return s
}

func (t *DNSTask) sanitizeDNSValue(v any) any {
	switch x := v.(type) {
	case string:
		return t.safeDNSString(x)
	case []string:
		out := make([]string, len(x))
		for i, s := range x {
			out[i] = t.safeDNSString(s)
		}
		return out
	default:
		return v
	}
}
