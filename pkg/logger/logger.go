package logger

import (
	"io"
	"log"
	"os"

	waLog "go.mau.fi/whatsmeow/util/log"
)

// Level represents the log level
type Level int

const (
	DEBUG Level = iota
	INFO
	WARN
	ERROR
)

// Logger is a custom logger implementation
type Logger struct {
	prefix string
	level  Level
	output io.Writer
}

// New creates a new logger instance
func New(prefix string, level Level) *Logger {
	return &Logger{
		prefix: prefix,
		level:  level,
		output: os.Stdout,
	}
}

// SetOutput sets the output destination for the logger
func (l *Logger) SetOutput(w io.Writer) {
	l.output = w
}

// Debug logs a debug message
func (l *Logger) Debug(args ...interface{}) {
	if l.level <= DEBUG {
		log.New(l.output, l.prefix+"[DEBUG] ", log.LstdFlags).Println(args...)
	}
}

// Debugf logs a formatted debug message
func (l *Logger) Debugf(format string, args ...interface{}) {
	if l.level <= DEBUG {
		log.New(l.output, l.prefix+"[DEBUG] ", log.LstdFlags).Printf(format, args...)
	}
}

// Info logs an info message
func (l *Logger) Info(args ...interface{}) {
	if l.level <= INFO {
		log.New(l.output, l.prefix+"[INFO] ", log.LstdFlags).Println(args...)
	}
}

// Infof logs a formatted info message
func (l *Logger) Infof(format string, args ...interface{}) {
	if l.level <= INFO {
		log.New(l.output, l.prefix+"[INFO] ", log.LstdFlags).Printf(format, args...)
	}
}

// Warn logs a warning message
func (l *Logger) Warn(args ...interface{}) {
	if l.level <= WARN {
		log.New(l.output, l.prefix+"[WARN] ", log.LstdFlags).Println(args...)
	}
}

// Warnf logs a formatted warning message
func (l *Logger) Warnf(format string, args ...interface{}) {
	if l.level <= WARN {
		log.New(l.output, l.prefix+"[WARN] ", log.LstdFlags).Printf(format, args...)
	}
}

// Error logs an error message
func (l *Logger) Error(args ...interface{}) {
	if l.level <= ERROR {
		log.New(l.output, l.prefix+"[ERROR] ", log.LstdFlags).Println(args...)
	}
}

// Errorf logs a formatted error message
func (l *Logger) Errorf(format string, args ...interface{}) {
	if l.level <= ERROR {
		log.New(l.output, l.prefix+"[ERROR] ", log.LstdFlags).Printf(format, args...)
	}
}

// Fatal logs a fatal error and exits
func (l *Logger) Fatal(args ...interface{}) {
	log.New(l.output, l.prefix+"[FATAL] ", log.LstdFlags).Fatalln(args...)
}

// Fatalf logs a formatted fatal error and exits
func (l *Logger) Fatalf(format string, args ...interface{}) {
	log.New(l.output, l.prefix+"[FATAL] ", log.LstdFlags).Fatalf(format, args...)
}

// Sub creates a sub-logger with an additional prefix
func (l *Logger) Sub(module string) waLog.Logger {
	return &WhatsAppLogger{
		logger: &Logger{
			prefix: l.prefix + "[" + module + "] ",
			level:  l.level,
			output: l.output,
		},
	}
}

// WhatsAppLogger adapts our logger to the WhatsApp logger interface
type WhatsAppLogger struct {
	logger *Logger
}

func (w *WhatsAppLogger) Debugf(format string, args ...interface{}) {
	w.logger.Debugf(format, args...)
}

func (w *WhatsAppLogger) Infof(format string, args ...interface{}) {
	w.logger.Infof(format, args...)
}

func (w *WhatsAppLogger) Warnf(format string, args ...interface{}) {
	w.logger.Warnf(format, args...)
}

func (w *WhatsAppLogger) Errorf(format string, args ...interface{}) {
	w.logger.Errorf(format, args...)
}

func (w *WhatsAppLogger) Sub(module string) waLog.Logger {
	return w.logger.Sub(module)
}

// NewWhatsAppLogger creates a new WhatsApp logger
func NewWhatsAppLogger(prefix string, level Level) waLog.Logger {
	return &WhatsAppLogger{
		logger: New(prefix, level),
	}
}
