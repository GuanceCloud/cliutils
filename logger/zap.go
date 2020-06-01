package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	OPT_ENC_CONSOLE  = 1
	OPT_SHORT_CALLER = 2

	LVL_DEBUG zapcore.Level = zapcore.DebugLevel
	LVL_INFO  zapcore.Level = zapcore.InfoLevel
)

func GetLogger(zl *zap.Logger, name string) *zap.Logger {
	return zl.Named(name)
}

func NewRootLogger(fpath string, options int, level zapcore.Level) (*zap.Logger, error) {
	cfg := zap.Config{
		Encoding:    `json`,
		Level:       zap.NewAtomicLevelAt(zapcore.DebugLevel),
		OutputPaths: []string{fpath},
		EncoderConfig: zapcore.EncoderConfig{
			NameKey:    "MOD",
			MessageKey: "MSG",

			LevelKey:    "LEV",
			EncodeLevel: zapcore.CapitalLevelEncoder,

			TimeKey:    "TS",
			EncodeTime: zapcore.ISO8601TimeEncoder,

			CallerKey:    "POS",
			EncodeCaller: zapcore.FullCallerEncoder,
		},
	}

	if options&OPT_ENC_CONSOLE != 0 {
		cfg.Encoding = "console"
	}

	if options&OPT_SHORT_CALLER != 0 {
		cfg.EncoderConfig.EncodeCaller = zapcore.ShortCallerEncoder
	}

	l, err := cfg.Build()
	if err != nil {
		return nil, err
	}

	return l, nil
}
