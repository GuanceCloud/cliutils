English | [简体中文](README_cn.md)

# Guance Cloud cliutils

This library mainly collects various common functional packages in the Guance Cloud. See the README of each package for details.

## Logger Package

The logger package provides structured logging capabilities with support for separate error log files.

### Error Logging Feature

The logger supports writing error-level logs to a separate file for better log organization and monitoring:

```go
import "github.com/GuanceCloud/cliutils/logger"

// Initialize logger with separate error file
opt := &logger.Option{
    Path:      "/var/log/app/main.log",    // Main log file
    ErrorPath: "/var/log/app/error.log",   // Separate error log file
    Level:     "debug",
    Flags:     logger.OPT_DEFAULT,
}

err := logger.InitRoot(opt)
if err != nil {
    panic(err)
}
```

**Key Features:**
- Error logs (`ERROR`, `PANIC`, `FATAL`, `DPANIC`) are written to both main log file and separate error file
- Main log file receives all logs up to configured level
- Error file receives only error-level and above logs
- Both files support automatic rotation with configurable size, backup count, and age
- Remote TCP/UDP logging ignores ErrorPath (all logs go to same endpoint)

**Configuration Options:**
- `Path`: Main log file path
- `ErrorPath`: Separate error log file path (optional)
- `Level`: Minimum log level (`debug`, `info`, `warn`, `error`, `panic`, `fatal`, `dpanic`)
- `Flags`: Bit flags for output options (`OPT_DEFAULT`, `OPT_COLOR`, `OPT_ROTATE`, etc.)
- `MaxSize`: Maximum file size in MB before rotation (default: 32MB)
- `MaxBackups`: Maximum number of old log files to retain (default: 5)
- `MaxAge`: Maximum number of days to retain old log files (default: 30)
- `Compress`: Compress rotated log files (default: false)

[![GoDoc](https://godoc.org/github.com/GuanceCloud/cliutils?status.svg)](https://godoc.org/github.com/GuanceCloud/cliutils)
[![MIT License](https://img.shields.io/badge/license-MIT-green?style=plastic)](LICENSE)
