package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Logger wraps zap.SugaredLogger for structured logging
type Logger struct {
	*zap.SugaredLogger
}

// New creates a new logger instance
func New(verbose bool) *Logger {
	var config zap.Config

	if verbose {
		// Verbose mode: show DEBUG and above
		config = zap.NewDevelopmentConfig()
		config.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	} else {
		// Non-verbose mode: show WARN and above (hide INFO and DEBUG)
		config = zap.NewProductionConfig()
		config.Level = zap.NewAtomicLevelAt(zap.WarnLevel)
	}

	// Use console encoder for better readability
	config.Encoding = "console"
	config.EncoderConfig.TimeKey = "time"
	config.EncoderConfig.LevelKey = "level"
	config.EncoderConfig.NameKey = "logger"
	config.EncoderConfig.CallerKey = "caller"
	config.EncoderConfig.MessageKey = "msg"
	config.EncoderConfig.StacktraceKey = "stacktrace"
	config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	config.EncoderConfig.EncodeCaller = zapcore.ShortCallerEncoder

	logger, err := config.Build()
	if err != nil {
		panic(err)
	}

	return &Logger{
		SugaredLogger: logger.Sugar(),
	}
}

// WithFields returns a logger with additional fields
func (l *Logger) WithFields(fields ...interface{}) *Logger {
	return &Logger{
		SugaredLogger: l.SugaredLogger.With(fields...),
	}
}

// WithWorkflow returns a logger with workflow context
func (l *Logger) WithWorkflow(workflowName string) *Logger {
	return l.WithFields("workflow", workflowName)
}

// WithJob returns a logger with job context
func (l *Logger) WithJob(jobName string) *Logger {
	return l.WithFields("job", jobName)
}

// WithStep returns a logger with step context
func (l *Logger) WithStep(stepName string) *Logger {
	return l.WithFields("step", stepName)
}
