package log

import (
	"fmt"
	"io"
	"os"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Options control CLI logging (stderr). Machine output stays on stdout.
type Options struct {
	// Devel enables a human-friendly development encoder (like --zap-devel).
	Devel bool
	// Level is a zap level name: debug, info, warn, error. Empty → info.
	Level string
	// Encoder is "json" or "console". Empty → console when Devel, else json.
	Encoder string
	// Out defaults to stderr.
	Out io.Writer
}

// Setup builds a logr.Logger writing to stderr (or Out).
func Setup(opts Options) (logr.Logger, *zap.Logger, error) {
	out := opts.Out
	if out == nil {
		out = os.Stderr
	}

	level := zapcore.InfoLevel
	if opts.Level != "" {
		if err := level.UnmarshalText([]byte(opts.Level)); err != nil {
			return logr.Logger{}, nil, fmt.Errorf("log level: %w", err)
		}
	}

	encoderName := opts.Encoder
	if encoderName == "" {
		if opts.Devel {
			encoderName = "console"
		} else {
			encoderName = "json"
		}
	}

	var encoder zapcore.Encoder
	switch encoderName {
	case "console":
		encCfg := zap.NewDevelopmentEncoderConfig()
		encCfg.EncodeTime = zapcore.ISO8601TimeEncoder
		encoder = zapcore.NewConsoleEncoder(encCfg)
	case "json":
		encCfg := zap.NewProductionEncoderConfig()
		encCfg.EncodeTime = zapcore.ISO8601TimeEncoder
		encoder = zapcore.NewJSONEncoder(encCfg)
	default:
		return logr.Logger{}, nil, fmt.Errorf("unknown log encoder %q (want json|console)", encoderName)
	}

	core := zapcore.NewCore(encoder, zapcore.AddSync(out), level)
	var z *zap.Logger
	if opts.Devel {
		z = zap.New(core, zap.Development(), zap.AddCaller())
	} else {
		z = zap.New(core)
	}
	return zapr.NewLogger(z), z, nil
}

// Discard returns a no-op logger for tests.
func Discard() logr.Logger {
	return logr.Discard()
}
