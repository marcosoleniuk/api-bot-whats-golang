package logger

import (
	"io"
	"log"
	"os"

	waLog "go.mau.fi/whatsmeow/util/log"
)

type Level int

const (
	DEBUG Level = iota
	INFO
	WARN
	ERROR
)

type Logger struct {
	prefix string
	level  Level
	output io.Writer
}

func New(prefix string, level Level) *Logger {
	return &Logger{
		prefix: prefix,
		level:  level,
		output: os.Stdout,
	}
}

func (l *Logger) SetOutput(w io.Writer) {
	l.output = w
}

func (l *Logger) Debug(args ...interface{}) {
	if l.level <= DEBUG {
		log.New(l.output, l.prefix+"[DEBUG] ", log.LstdFlags).Println(args...)
	}
}

func (l *Logger) Debugf(format string, args ...interface{}) {
	if l.level <= DEBUG {
		log.New(l.output, l.prefix+"[DEBUG] ", log.LstdFlags).Printf(format, args...)
	}
}

func (l *Logger) Info(args ...interface{}) {
	if l.level <= INFO {
		log.New(l.output, l.prefix+"[INFO] ", log.LstdFlags).Println(args...)
	}
}

func (l *Logger) Infof(format string, args ...interface{}) {
	if l.level <= INFO {
		log.New(l.output, l.prefix+"[INFO] ", log.LstdFlags).Printf(format, args...)
	}
}

func (l *Logger) Warn(args ...interface{}) {
	if l.level <= WARN {
		log.New(l.output, l.prefix+"[WARN] ", log.LstdFlags).Println(args...)
	}
}

func (l *Logger) Warnf(format string, args ...interface{}) {
	if l.level <= WARN {
		log.New(l.output, l.prefix+"[WARN] ", log.LstdFlags).Printf(format, args...)
	}
}

func (l *Logger) Error(args ...interface{}) {
	if l.level <= ERROR {
		log.New(l.output, l.prefix+"[ERROR] ", log.LstdFlags).Println(args...)
	}
}

func (l *Logger) Errorf(format string, args ...interface{}) {
	if l.level <= ERROR {
		log.New(l.output, l.prefix+"[ERROR] ", log.LstdFlags).Printf(format, args...)
	}
}

func (l *Logger) Fatal(args ...interface{}) {
	log.New(l.output, l.prefix+"[FATAL] ", log.LstdFlags).Fatalln(args...)
}

func (l *Logger) Fatalf(format string, args ...interface{}) {
	log.New(l.output, l.prefix+"[FATAL] ", log.LstdFlags).Fatalf(format, args...)
}

func (l *Logger) Sub(module string) waLog.Logger {
	return &WhatsAppLogger{
		logger: &Logger{
			prefix: l.prefix + "[" + module + "] ",
			level:  l.level,
			output: l.output,
		},
	}
}

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

func NewWhatsAppLogger(prefix string, level Level) waLog.Logger {
	return &WhatsAppLogger{
		logger: New(prefix, level),
	}
}
