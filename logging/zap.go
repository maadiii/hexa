package logging

import (
	"io"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Options struct {
	Service string
	Version string
	Level   zapcore.Level
	Format  string // "text" for humans, "json" for log collectors
}

func New(w io.Writer, o Options) *zap.Logger {
	var enc zapcore.Encoder

	if o.Format == "json" {
		cfg := zap.NewProductionEncoderConfig()
		cfg.EncodeTime = zapcore.ISO8601TimeEncoder
		enc = zapcore.NewJSONEncoder(cfg)
	} else {
		cfg := zap.NewDevelopmentEncoderConfig()
		cfg.EncodeTime = shortTime
		enc = zapcore.NewConsoleEncoder(cfg)
	}

	core := zapcore.NewCore(enc, zapcore.AddSync(w), o.Level)

	return zap.New(core).With(
		zap.String("service", o.Service),
		zap.String("version", o.Version),
	)
}

func ParseLevel(s string) (zapcore.Level, error) {
	var l zapcore.Level
	err := l.UnmarshalText([]byte(s))

	return l, err
}

func shortTime(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(t.Format("15:04:05.000"))
}
