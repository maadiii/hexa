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

	SampleTick       time.Duration
	SampleFirst      int
	SampleThereafter int

	BufferSize    int
	FlushInterval time.Duration
}

func (o Options) withDefaults() Options {
	if o.SampleTick == 0 {
		o.SampleTick = time.Second
	}
	if o.SampleFirst == 0 {
		o.SampleFirst = 100
	}
	if o.SampleThereafter == 0 {
		o.SampleThereafter = 100
	}
	if o.BufferSize == 0 {
		o.BufferSize = 256 * 1024 //nolint
	}
	if o.FlushInterval == 0 {
		o.FlushInterval = 3 * time.Second //nolint
	}

	return o
}

func New(w io.Writer, o Options) (*zap.Logger, func()) {
	o = o.withDefaults()

	ws := &zapcore.BufferedWriteSyncer{
		WS:            zapcore.AddSync(w),
		Size:          o.BufferSize,
		FlushInterval: o.FlushInterval,
	}

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

	lowPriority := zap.LevelEnablerFunc(func(l zapcore.Level) bool {
		return l >= o.Level && l < zapcore.ErrorLevel
	})
	highPriority := zap.LevelEnablerFunc(func(l zapcore.Level) bool {
		return l >= o.Level && l >= zapcore.ErrorLevel
	})

	core := zapcore.NewTee(
		zapcore.NewSamplerWithOptions(
			zapcore.NewCore(enc, ws, lowPriority),
			o.SampleTick, o.SampleFirst, o.SampleThereafter,
		),
		zapcore.NewCore(enc, ws, highPriority),
	)

	logger := zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.DPanicLevel)).With(
		zap.String("service", o.Service),
		zap.String("version", o.Version),
	)

	closeFn := func() {
		_ = logger.Sync()
		_ = ws.Stop()
	}

	return logger, closeFn
}

func shortTime(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(t.Format("15:04:05.000"))
}
