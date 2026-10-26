// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package dialtesting

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"

	"golang.org/x/net/http/httpguts"
	"golang.org/x/net/idna"
)

const dnsMetricName = "dns_dial_testing"

var dnsRecordTypes = []string{"A", "AAAA", "CNAME", "MX", "NS", "TXT"}

// DNSDoHOption configures a standard DNS-over-HTTPS endpoint.
type DNSDoHOption struct {
	URL          string            `json:"url"`
	Method       string            `json:"method,omitempty"`
	HTTPProtocol string            `json:"http_protocol,omitempty"`
	Headers      map[string]string `json:"headers,omitempty"`
}

// DNSQueryOptions configures DNS request flags and EDNS0.
type DNSQueryOptions struct {
	RecursionDesired *bool `json:"recursion_desired,omitempty"`
	DNSSECOK         bool  `json:"dnssec_ok,omitempty"`
	CheckingDisabled bool  `json:"checking_disabled,omitempty"`
	UDPPayloadSize   int   `json:"udp_payload_size,omitempty"`
}

// DNSRecordCondition compares a normalized record value or tests its existence.
type DNSRecordCondition struct {
	SuccessOption
	Exists bool `json:"exists,omitempty"`
}

// DNSRecordAssertions groups conditions by record type.
type DNSRecordAssertions struct {
	A     []*DNSRecordCondition `json:"of_type_a,omitempty"`
	AAAA  []*DNSRecordCondition `json:"of_type_aaaa,omitempty"`
	CNAME []*DNSRecordCondition `json:"of_type_cname,omitempty"`
	MX    []*DNSRecordCondition `json:"of_type_mx,omitempty"`
	NS    []*DNSRecordCondition `json:"of_type_ns,omitempty"`
	TXT   []*DNSRecordCondition `json:"of_type_txt,omitempty"`
}

func (r *DNSRecordAssertions) conditions() map[string][]*DNSRecordCondition {
	if r == nil {
		return nil
	}
	return map[string][]*DNSRecordCondition{"A": r.A, "AAAA": r.AAAA, "CNAME": r.CNAME, "MX": r.MX, "NS": r.NS, "TXT": r.TXT}
}

// DNSSuccess uses the same flat AND/OR semantics as other synthetic tests.
type DNSSuccess struct {
	ResponseTime      string               `json:"response_time,omitempty"`
	AtLeastOneRecord  *DNSRecordAssertions `json:"at_least_one_record,omitempty"`
	EveryRecord       *DNSRecordAssertions `json:"every_record,omitempty"`
	AuthenticatedData *bool                `json:"authenticated_data,omitempty"`
}

// DNSTask probes one logical DNS endpoint using ordinary or encrypted DNS.
type DNSTask struct {
	*Task
	Domain           string              `json:"domain"`
	DNSServer        string              `json:"dns_server"`
	ServerIP         string              `json:"server_ip,omitempty"`
	ServerName       string              `json:"server_name,omitempty"`
	Port             string              `json:"port,omitempty"`
	Timeout          string              `json:"timeout,omitempty"`
	Protocol         string              `json:"protocol,omitempty"`
	RecordTypes      []string            `json:"record_types,omitempty"`
	Certificate      *HTTPOptCertificate `json:"certificate,omitempty"`
	DoH              *DNSDoHOption       `json:"doh,omitempty"`
	DNSOptions       *DNSQueryOptions    `json:"dns_options,omitempty"`
	SuccessWhen      []*DNSSuccess       `json:"success_when"`
	SuccessWhenLogic string              `json:"success_when_logic,omitempty"`

	mu         sync.Mutex
	cancel     context.CancelFunc
	stopped    bool
	validator  func(context.Context, net.IP) error
	resolve    func(context.Context, string) ([]net.IPAddr, error)
	timeout    time.Duration
	host       string
	queryTypes []string
	result     dnsRunResult
	rawConfig  string
}

var _ TaskChild = (*DNSTask)(nil)

// SetResolvedIPValidator checks the actual address used by every DNS transport.
// The validator must be installed before execution; it is not serialized.
func (t *DNSTask) SetResolvedIPValidator(f func(context.Context, net.IP) error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.validator = f
}

func dnsName(s string) (string, error) {
	s = strings.TrimSuffix(strings.TrimSpace(s), ".")
	if s == "" {
		return "", errors.New("DNS name must not be empty")
	}
	labels := strings.Split(s, ".")
	for i, label := range labels {
		var err error
		if !strings.Contains(label, "_") {
			label, err = idna.Lookup.ToASCII(label)
		}
		if err != nil || len(label) == 0 || len(label) > 63 {
			return "", errors.New("invalid DNS name label")
		}
		for _, ch := range label {
			if !(ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9' || ch == '-' || ch == '_') {
				return "", errors.New("invalid DNS name character")
			}
		}
		labels[i] = strings.ToLower(label)
	}
	s = strings.Join(labels, ".")
	if len(s) > 253 {
		return "", errors.New("DNS name is too long")
	}
	return s, nil
}

func (t *DNSTask) check() error { return t.prepare(true) }
func (t *DNSTask) init() error  { return t.prepare(true) }

func (t *DNSTask) prepare(allowTemplate bool) error {
	templated := func(s string) bool { return allowTemplate && hasTemplateTags(s) }
	var err error
	if _, err = dnsName(t.Domain); !templated(t.Domain) && err != nil {
		return fmt.Errorf("domain: %w", err)
	}
	if t.Protocol == "" {
		t.Protocol = "auto"
	}
	switch t.Protocol {
	case "auto", "udp", "tcp", "dot", "doh", "doq":
	default:
		if !templated(t.Protocol) {
			return errors.New("invalid DNS protocol")
		}
	}
	if t.Timeout == "" {
		t.Timeout = "10s"
	}
	t.timeout, err = time.ParseDuration(t.Timeout)
	if !templated(t.Timeout) && (err != nil || t.timeout <= 0 || t.timeout > time.Minute) {
		return errors.New("DNS timeout must be in (0, 60s]")
	}
	if !templated(t.SuccessWhenLogic) && t.SuccessWhenLogic != "" && t.SuccessWhenLogic != "and" && t.SuccessWhenLogic != "or" {
		return errors.New("invalid DNS success_when_logic")
	}
	t.host = t.DNSServer
	if t.Protocol == "doh" {
		if err = t.prepareDoH(allowTemplate); err != nil {
			return err
		}
	} else if t.Port == "" && !templated(t.Protocol) {
		if t.Protocol == "dot" || t.Protocol == "doq" {
			t.Port = "853"
		} else {
			t.Port = "53"
		}
	}
	deferredEndpoint := templated(t.Protocol) || (t.Protocol == "doh" && t.DoH != nil && templated(t.DoH.URL))
	if !templated(t.host) && !(t.host == "" && deferredEndpoint) {
		if ip, e := netip.ParseAddr(t.host); e == nil {
			if ip.Zone() != "" {
				return errors.New("scoped DNS server addresses are not supported")
			}
			t.host = ip.String()
		} else if t.host, err = dnsName(t.host); err != nil {
			return fmt.Errorf("dns_server: %w", err)
		}
	}
	if !templated(t.Port) && !(t.Port == "" && deferredEndpoint) {
		port, e := strconv.Atoi(t.Port)
		if e != nil || port < 1 || port > 65535 {
			return errors.New("DNS port must be in [1, 65535]")
		}
		if (t.Protocol == "dot" || t.Protocol == "doq") && port == 53 {
			return errors.New("DoT and DoQ cannot use port 53")
		}
	}
	if t.ServerIP != "" && !templated(t.ServerIP) {
		ip, e := netip.ParseAddr(t.ServerIP)
		if e != nil || ip.Zone() != "" {
			return errors.New("server_ip must be an IP address without a zone")
		}
	}
	if t.ServerName != "" && !templated(t.ServerName) {
		if ip := net.ParseIP(t.ServerName); ip != nil {
			t.ServerName = ip.String()
		} else {
			name, e := dnsName(t.ServerName)
			if e != nil {
				return errors.New("invalid DNS server_name")
			}
			t.ServerName = name
		}
	}
	if t.DNSOptions == nil {
		t.DNSOptions = &DNSQueryOptions{}
	}
	if t.DNSOptions.UDPPayloadSize == 0 {
		t.DNSOptions.UDPPayloadSize = 1232
	}
	if t.DNSOptions.UDPPayloadSize < 512 || t.DNSOptions.UDPPayloadSize > 4096 {
		return errors.New("UDP payload size must be in [512, 4096]")
	}
	if t.Certificate != nil && (t.Certificate.Certificate == "") != (t.Certificate.PrivateKey == "") {
		return errors.New("client certificate and private key must be provided together")
	}
	return t.prepareAssertions(allowTemplate)
}

func (t *DNSTask) prepareDoH(allowTemplate bool) error {
	templated := func(s string) bool { return allowTemplate && hasTemplateTags(s) }
	if t.DoH == nil {
		return errors.New("doh.url is required")
	}
	if !templated(t.DoH.URL) {
		u, err := url.Parse(t.DoH.URL)
		if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Fragment != "" {
			return errors.New("doh.url must be an HTTPS URL without userinfo or fragment")
		}
		host, err := dnsName(u.Hostname())
		if ip := net.ParseIP(u.Hostname()); ip != nil {
			host, err = ip.String(), nil
		}
		if err != nil {
			return errors.New("invalid DoH host")
		}
		if t.DNSServer != "" && !templated(t.DNSServer) {
			server, e := dnsName(t.DNSServer)
			if ip := net.ParseIP(t.DNSServer); ip != nil {
				server, e = ip.String(), nil
			}
			if e != nil || server != host {
				return errors.New("dns_server must match doh.url host")
			}
		}
		t.host = host
		port := u.Port()
		if port == "" {
			port = "443"
		}
		if t.Port != "" && !templated(t.Port) && t.Port != port {
			return errors.New("port must match doh.url port")
		}
		if !templated(t.Port) {
			t.Port = port
		}
	}
	if t.DoH.Method == "" {
		t.DoH.Method = "POST"
	}
	if !templated(t.DoH.Method) && t.DoH.Method != "POST" && t.DoH.Method != "GET" {
		return errors.New("DoH method must be GET or POST")
	}
	u, _ := url.Parse(t.DoH.URL)
	if !templated(t.DoH.URL) && t.DoH.Method == "GET" && u.Query().Has("dns") {
		return errors.New("DoH URL must not contain a dns parameter for GET")
	}
	switch t.DoH.HTTPProtocol {
	case "", "auto", "http/1.1", "http/2-only", "http/3":
	default:
		if !templated(t.DoH.HTTPProtocol) {
			return errors.New("invalid DoH HTTP protocol")
		}
	}
	for k, v := range t.DoH.Headers {
		switch strings.ToLower(k) {
		case "host", "content-length", "content-type", "accept", "connection", "transfer-encoding", "upgrade", "te", "trailer":
			return fmt.Errorf("reserved DoH header %q", k)
		}
		if !httpguts.ValidHeaderFieldName(k) || !httpguts.ValidHeaderFieldValue(v) {
			return errors.New("invalid DoH header")
		}
	}
	return nil
}

func (t *DNSTask) initTask() {
	if t.Task == nil {
		t.Task = &Task{}
	}
}
func (t *DNSTask) class() string      { return ClassDNS }
func (t *DNSTask) metricName() string { return dnsMetricName }
func (t *DNSTask) clear() {
	t.result = dnsRunResult{}
	// Secure variable values can change between runs.
	t.Task.rawTask = ""
}
func (t *DNSTask) setReqError(s string) { t.result.Code, t.result.Error = "address_denied", s }
func (t *DNSTask) stop() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.stopped = true
	if t.cancel != nil {
		t.cancel()
	}
}
func (t *DNSTask) getHostName() ([]string, error) {
	if err := t.prepare(true); err != nil {
		return nil, err
	}
	if t.ServerIP != "" {
		return []string{t.ServerIP}, nil
	}
	return []string{t.host}, nil
}
func (t *DNSTask) getVariableValue(Variable) (string, error) {
	return "", errors.New("DNS variable extraction is not supported")
}

func (t *DNSTask) renderTemplate(fm template.FuncMap) error {
	if t.rawConfig == "" {
		t.rawConfig = t.GetTaskJSONString()
	}
	var obj any
	if err := json.Unmarshal([]byte(t.rawConfig), &obj); err != nil {
		return err
	}
	var render func(any) (any, error)
	render = func(v any) (any, error) {
		switch x := v.(type) {
		case string:
			return t.GetParsedString(x, fm)
		case []any:
			for i := range x {
				var err error
				x[i], err = render(x[i])
				if err != nil {
					return nil, err
				}
			}
		case map[string]any:
			for k, val := range x {
				if k == "config_vars" {
					continue
				}
				y, err := render(val)
				if err != nil {
					return nil, err
				}
				x[k] = y
			}
		}
		return v, nil
	}
	value, err := render(obj)
	if err != nil {
		return err
	}
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	var next DNSTask
	if err = json.Unmarshal(data, &next); err != nil {
		return err
	}
	t.Domain, t.DNSServer, t.ServerIP, t.ServerName = next.Domain, next.DNSServer, next.ServerIP, next.ServerName
	t.Port, t.Timeout, t.Protocol, t.RecordTypes = next.Port, next.Timeout, next.Protocol, next.RecordTypes
	t.Certificate, t.DoH, t.DNSOptions = next.Certificate, next.DoH, next.DNSOptions
	t.SuccessWhen, t.SuccessWhenLogic = next.SuccessWhen, next.SuccessWhenLogic
	return t.prepare(false)
}

func (t *DNSTask) getRawTask(s string) (string, error) {
	var raw DNSTask
	if err := json.Unmarshal([]byte(s), &raw); err != nil {
		return "", err
	}
	raw.Task = nil
	if raw.Certificate != nil {
		if raw.Certificate.PrivateKey != "" {
			raw.Certificate.PrivateKey = "[redacted]"
		}
		if raw.Certificate.Certificate != "" {
			raw.Certificate.Certificate = "[redacted]"
		}
		if raw.Certificate.CaCert != "" {
			raw.Certificate.CaCert = "[redacted]"
		}
	}
	if raw.DoH != nil {
		for k := range raw.DoH.Headers {
			raw.DoH.Headers[k] = "[redacted]"
		}
		if u, err := url.Parse(raw.DoH.URL); err == nil {
			u.RawQuery, u.User = "", nil
			raw.DoH.URL = u.String()
		}
	}
	b, err := json.Marshal(&raw)
	if err != nil {
		return "", err
	}
	// Redact string values before encoding so escaping and JSON syntax remain intact.
	var config any
	if err = json.Unmarshal(b, &config); err != nil {
		return "", err
	}
	var redact func(any) any
	redact = func(value any) any {
		switch v := value.(type) {
		case string:
			return t.redactDNSString(v)
		case map[string]any:
			for key, item := range v {
				v[key] = redact(item)
			}
		case []any:
			for i, item := range v {
				v[i] = redact(item)
			}
		}
		return value
	}
	b, err = json.Marshal(redact(config))
	return string(b), err
}

func (t *DNSTask) run() error {
	if err := t.prepare(false); err != nil {
		t.result.Code, t.result.Error = "invalid_response", "invalid DNS configuration"
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), t.timeout)
	t.mu.Lock()
	if t.stopped {
		cancel()
	} else {
		t.cancel = cancel
	}
	t.mu.Unlock()
	defer func() { cancel(); t.mu.Lock(); t.cancel = nil; t.mu.Unlock() }()
	start := time.Now()
	defer func() { t.result.Cost = time.Since(start) }()
	if err := t.selectAddress(ctx); err != nil {
		t.setDNSError(ctx, "bootstrap_error", err)
		t.skipQueries()
		return nil
	}
	t.result.Started = true
	client, err := t.newDNSClient(ctx)
	if err != nil {
		code := "tls_error"
		if t.Protocol == "doq" {
			code = "quic_error"
		}
		t.setDNSError(ctx, code, err)
		t.skipQueries()
		return nil
	}
	defer client.close()
	for _, typ := range t.queryTypes {
		if ctx.Err() != nil && t.result.Error == "" {
			t.setDNSError(ctx, "timeout", ctx.Err())
		}
		if t.result.Error != "" {
			t.result.Queries = append(t.result.Queries, dnsQueryResult{Type: typ, Status: "SKIPPED", Error: "previous query failed or budget exhausted"})
			continue
		}
		query, err := t.exchangeDNS(ctx, client, typ)
		t.result.Queries = append(t.result.Queries, query)
		if err != nil {
			t.setDNSError(ctx, query.Code, err)
		}
	}
	if err := ctx.Err(); err != nil && t.result.Error == "" {
		t.setDNSError(ctx, "timeout", err)
	}
	return nil
}

func (t *DNSTask) selectAddress(ctx context.Context) error {
	t.mu.Lock()
	validate := t.validator
	t.mu.Unlock()
	t.result.BootstrapSource = "system_resolver"
	ip := net.ParseIP(t.ServerIP)
	if ip != nil {
		t.result.BootstrapSource = "server_ip"
	}
	if ip == nil {
		ip = net.ParseIP(t.host)
		if ip != nil {
			t.result.BootstrapSource = "literal_ip"
		}
	}
	var candidates []net.IPAddr
	if ip != nil {
		candidates = []net.IPAddr{{IP: ip}}
	} else {
		start := time.Now()
		resolve := t.resolve
		if resolve == nil {
			resolve = net.DefaultResolver.LookupIPAddr
		}
		var err error
		t.result.Started, t.result.BootstrapStarted = true, true
		candidates, err = resolve(ctx, t.host)
		t.result.Bootstrap = time.Since(start)
		if err != nil {
			return err
		}
	}
	var denied error
	for _, candidate := range candidates {
		if candidate.IP == nil || candidate.IP.To16() == nil {
			continue
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if validate != nil {
			if err := validate(ctx, candidate.IP); err != nil {
				denied = err
				continue
			}
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		t.result.IP = candidate.IP.String()
		return nil
	}
	if denied != nil {
		t.result.Code = "address_denied"
		return denied
	}
	return errors.New("DNS server has no usable addresses")
}

func (t *DNSTask) setDNSError(ctx context.Context, code string, err error) {
	if t.result.Code == "address_denied" {
		code = "address_denied"
	}
	if ctx.Err() == context.DeadlineExceeded {
		code = "timeout"
	} else if ctx.Err() == context.Canceled {
		code = "canceled"
	}
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		code = "timeout"
	}
	if code == "" {
		code = "network_error"
	}
	t.result.Code, t.result.Error = code, dnsErrorMessage(err)
}

func (t *DNSTask) skipQueries() {
	for _, typ := range t.queryTypes {
		t.result.Queries = append(t.result.Queries, dnsQueryResult{Type: typ, Status: "SKIPPED", Error: "endpoint preparation failed"})
	}
}

// HTTP errors wrap the request URL, which can contain authentication parameters.
func dnsErrorMessage(err error) string {
	var ue *url.Error
	if errors.As(err, &ue) {
		err = ue.Err
	}
	return err.Error()
}
