package logger

import (
	"os"
	"runtime"
	"strings"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Logger struct {
	*zap.SugaredLogger
	name string
}

// NewLogger 创建新的日志记录器
func NewLogger(name string) *Logger {
	// 配置编码器
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "timestamp",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "message",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	// 设置日志级别
	level := zap.InfoLevel
	if envLevel := strings.ToLower(os.Getenv("LOG_LEVEL")); envLevel != "" {
		switch envLevel {
		case "debug":
			level = zap.DebugLevel
		case "info":
			level = zap.InfoLevel
		case "warn":
			level = zap.WarnLevel
		case "error":
			level = zap.ErrorLevel
		case "fatal":
			level = zap.FatalLevel
		}
	}

	// 创建核心
	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderConfig),
		zapcore.AddSync(os.Stdout),
		level,
	)

	// 创建 logger
	zapLogger := zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1))
	sugaredLogger := zapLogger.Sugar()

	return &Logger{
		SugaredLogger: sugaredLogger,
		name:          name,
	}
}

// WithFields 添加字段到日志
func (l *Logger) WithFields(fields map[string]interface{}) *Logger {
	zapFields := make([]interface{}, 0, len(fields)*2)
	for k, v := range fields {
		zapFields = append(zapFields, k, v)
	}

	return &Logger{
		SugaredLogger: l.SugaredLogger.With(zapFields...),
		name:          l.name,
	}
}

// Debug 调试日志
func (l *Logger) Debug(msg string, keysAndValues ...interface{}) {
	l.SugaredLogger.Debugw(msg, l.addDefaultFields(keysAndValues)...)
}

// Info 信息日志
func (l *Logger) Info(msg string, keysAndValues ...interface{}) {
	l.SugaredLogger.Infow(msg, l.addDefaultFields(keysAndValues)...)
}

// Warn 警告日志
func (l *Logger) Warn(msg string, keysAndValues ...interface{}) {
	l.SugaredLogger.Warnw(msg, l.addDefaultFields(keysAndValues)...)
}

// Error 错误日志
func (l *Logger) Error(msg string, keysAndValues ...interface{}) {
	l.SugaredLogger.Errorw(msg, l.addDefaultFields(keysAndValues)...)
}

// Fatal 致命错误日志
func (l *Logger) Fatal(msg string, keysAndValues ...interface{}) {
	l.SugaredLogger.Fatalw(msg, l.addDefaultFields(keysAndValues)...)
}

// 添加默认字段
func (l *Logger) addDefaultFields(keysAndValues []interface{}) []interface{} {
	// 获取调用者信息
	_, file, line, ok := runtime.Caller(2)
	caller := ""
	if ok {
		// 只保留文件名，不包含完整路径
		parts := strings.Split(file, "/")
		if len(parts) > 0 {
			caller = parts[len(parts)-1] + ":" + string(rune(line))
		}
	}

	// 添加默认字段
	defaultFields := []interface{}{
		"logger", l.name,
		"timestamp", time.Now().Format(time.RFC3339),
	}

	if caller != "" {
		defaultFields = append(defaultFields, "caller", caller)
	}

	return append(defaultFields, keysAndValues...)
}

// Sync 刷新日志缓冲区
func (l *Logger) Sync() error {
	return l.SugaredLogger.Sync()
}
