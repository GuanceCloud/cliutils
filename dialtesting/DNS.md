# DNS synthetic tasks

`CreateTaskChild("dns")` and `CreateTaskChild(ClassDNS)` create a `DNSTask`.
It uses the existing `NewTask`, `Check`, `RenderTemplateAndInit`, `Run`, and
`GetResults` lifecycle. Its measurement is `dns_dial_testing`.

```json
{
  "external_id": "dns-example",
  "name": "Example DNS",
  "status": "OK",
  "frequency": "1m",
  "domain": "www.example.com",
  "dns_server": "resolver.example.net",
  "protocol": "auto",
  "timeout": "10s",
  "success_when": [
    {"response_time": "500ms"},
    {"at_least_one_record": {"of_type_a": [{"is": "203.0.113.10"}]}}
  ],
  "success_when_logic": "and"
}
```

Supported transports are `auto`, `udp`, `tcp`, `dot`, `doh`, and `doq`.
`auto` retries a truncated UDP response over TCP at the same IP and port.
Timeouts never trigger protocol or resolver fallback. Plain DNS defaults to
port 53; DoT and DoQ default to 853 and reject port 53.

Resolver endpoints accept hostnames, IPv4, or IPv6. Optional `server_ip` pins
the connection address without changing the logical hostname used by TLS or
HTTP. Scoped IPv6 addresses are rejected. The host application can install
`SetResolvedIPValidator` to check actual addresses before any connection opens.
Bootstrap, connections, handshakes, all record types, and TCP fallback share
one timeout in `(0, 60s]`. `Stop` cancels an active run.

DoH requires `doh.url`, an HTTPS URL. `doh.method` is `POST` (default) or `GET`;
`doh.http_protocol` accepts `auto`, `http/1.1`, `http/2-only`, or `http/3`.
The HTTP authority stays equal to the URL host, including when `server_ip` or
`server_name` is supplied. Redirects and environment proxies are not used.
DoQ uses TLS 1.3, ALPN `doq`, and one DNS exchange per bidirectional stream.

Encrypted transports verify certificates by default. `certificate` uses the
existing `HTTPOptCertificate` JSON fields: `ca`, `certificate`, `private_key`,
and `ignore_server_certificate_error`. Private keys, certificate material,
DoH authentication headers, URL query parameters, and secure variable values
are removed from report details.

Record types are A, AAAA, CNAME, MX, NS, and TXT. Optional `record_types`
selects queries; otherwise record conditions determine the types, defaulting
to A when only timing or AD conditions are present. Conditions require 1–20
leaves, combined by `success_when_logic` (`and` by default, or `or`).
Record scopes are `at_least_one_record` and `every_record`, grouped under
`of_type_a`, `of_type_aaaa`, `of_type_cname`, `of_type_mx`, `of_type_ns`, or
`of_type_txt`. Each record condition specifies exactly one of `exists: true`,
`is`, `contains`, `match_regex`, or `not_match_regex`. Both scopes fail on an
empty record set. Equality normalizes IPs and DNS names; TXT is case sensitive.

All queries must produce complete matching responses with RCODE `NOERROR`.
Protocol failures cannot be overridden by `or`; their assertions are skipped.
`authenticated_data` checks the resolver's AD declaration across all queries.
`dns_options` supports RD, DO, CD, and EDNS UDP payload size (default 1232).
This is not a local DNSSEC signature validator.

One run produces one result, with `status=OK/FAIL` and `success=1/-1`.
Durations are integer microseconds. `dns_queries` and `assertion_results` are
JSON strings; result display limits do not affect assertion evaluation.
Successful results omit `fail_reason` for compatibility with existing debug APIs.

Run the local protocol and regression tests without a public DNS resolver:

```sh
go test -race ./dialtesting -run '^TestDNS' -count=20 -timeout=120s
go test ./dialtesting -count=1 -timeout=120s
```
