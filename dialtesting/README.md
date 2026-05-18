<!--
Unless explicitly stated otherwise all files in this repository are licensed
under the MIT License.
This product includes software developed at Guance Cloud (https://www.guance.com/).
Copyright 2021-present Guance, Inc.
-->

# Dialtesting Browser Tasks

`BROWSER` tasks run browser synthetic checks through an external `browser-dial`
process. The `dialtesting` package keeps the same task lifecycle as other dial
types:

```text
task JSON -> NewTask -> Run -> GetResults -> report
```

`browser-dial` executes the browser script and returns JSON. `dialtesting`
parses that JSON and reports the final `browser_dial_testing` point through the
existing reporting path.

## Runtime Dependencies

Browser tasks require these binaries on the dial node:

- `browser-dial`
- Lightpanda, resolved by `browser-dial`

`dialtesting` resolves `browser-dial` in this order:

1. `Task.SetOption()["browser_dial_path"]`
2. `Task.SetOption()["browserDialPath"]`
3. `BROWSER_DIAL_PATH`
4. `browser-dial` from `PATH`

Lightpanda is not configured in task JSON. Configure it in the node environment,
for example with `LIGHTPANDA_EXECUTABLE_PATH`, or make it available from `PATH`.

## Task Fields

Browser tasks use the normal common task fields, including:

- `external_id`
- `name`
- `status`
- `frequency`
- `schedule_type`
- `crontab`
- `post_url`
- `tags`
- `config_vars`
- `owner_external_id`

The browser-specific task field is:

```json
{
  "browser_config": "<browser-dial YAML script>"
}
```

`browser_config` is a YAML string using the native `browser-dial` script format.
It can contain `name`, `target`, `timeout_ms`, `tags`, `metadata`, `auth`,
`config_vars`, and `steps`.

Do not put these browser script fields at the outer task level:

- `target`
- `steps`
- `auth`
- `success_when`
- `success_when_logic`
- `browser_dial_path`
- `lightpanda_path`

Success rules belong in `browser_config.steps` as browser assertions such as
`assert_title`, `assert_url`, and `assert_text`.

## Example

```json
{
  "external_id": "bd-homepage",
  "name": "homepage",
  "status": "OK",
  "frequency": "1m",
  "schedule_type": "frequency",
  "tags": {
    "owner": "platform"
  },
  "browser_config": "name: homepage\ntarget: https://example.com\ntimeout_ms: 30000\nsteps:\n  - name: open homepage\n    action: goto\n  - name: check title\n    action: assert_title\n    contains: Example\n  - name: check body\n    action: assert_text\n    selector: body\n    contains: Example Domain\n"
}
```

The generated command is equivalent to:

```bash
browser-dial run <tmp-browser-config.yaml> --dry-run --skip-token-check --json
```

## Host Validation

`GetHostName()` returns every explicitly configured browser destination so the
caller can reject illegal dial addresses before execution.

It collects hostnames from:

- top-level `browser_config.target`
- `browser_config.steps` entries where `action: goto` and `url` is set

The returned list is de-duplicated. Runtime redirects, JavaScript navigation,
third-party page assets, and `eval`-generated requests are not statically
expanded.

## Result Mapping

When `browser-dial` returns `run.success=true`:

- tag `status=OK`
- field `success=1`

When `browser-dial` returns `run.success=false`:

- tag `status=FAIL`
- field `success=-1`
- field `fail_reason` from `run.fail_reason` and the error message

Common result fields include:

- `response_time`
- `last_step`
- `browser_run_id`
- `exit_code`
- `message`
- `page_url`
- `page_title`
- `trace_id`
- `trace_ids`
- `steps`
