package logger

import (
	"fmt"
	"log"
	"os"

	"github.com/logrusorgru/aurora"
)

// Logger is a sugar coating around log.Logger
type Logger struct {
	debug  bool
	stdout *log.Logger
	stderr *log.Logger
	errors int
	au     aurora.Aurora
}

var logger *Logger

// NewLogger returns *logger.Logger
func NewLogger(debug, noColor bool) *Logger {
	if logger == nil {
		lg := Logger{
			debug:  debug,
			stdout: log.New(os.Stdout, "", 0),
			stderr: log.New(os.Stderr, "", 0),
			errors: 0,
			au:     aurora.NewAurora(!noColor), // flip noColor. --no-color -> noColor=true therefore colors=!noColor=false
		}
		logger = &lg
	}
	return logger
}

// Debug prints to stdout only if --debug is set
func (l *Logger) Debug(args ...interface{}) {
	if l.debug {
		l.Info(args...)
	}

}

// Debugf prints formatted text to stdout only if --debug is set
func (l *Logger) Debugf(format string, args ...interface{}) {
	if l.debug {
		l.Infof(format, args...)
	}

}

// Info prints to stdout
func (l *Logger) Info(args ...interface{}) {
	l.stdout.Println(args...)
}

// Infof prints formatted text to stdout
func (l *Logger) Infof(format string, args ...interface{}) {
	l.stderr.Printf(format, args...)
}

// Warn prints to stderr
func (l *Logger) Warn(args ...interface{}) {
	l.errors++
	l.stderr.Println(l.au.Yellow(fmt.Sprint(args...)))
}

// Warnf prints to stderr
func (l *Logger) Warnf(format string, args ...interface{}) {
	l.errors++
	l.stderr.Print(l.au.Yellow(fmt.Sprintf(format, args...)))
}

// Error prints to stderr
func (l *Logger) Error(args ...interface{}) {
	l.errors++
	l.stderr.Println(l.au.Red(fmt.Sprint(args...)))
}

// Errorf prints to stderr
func (l *Logger) Errorf(format string, args ...interface{}) {
	l.errors++
	l.stderr.Print(l.au.Red(fmt.Sprintf(format, args...)))
}

// Fatal prints to stderr and exits 1
func (l *Logger) Fatal(args ...interface{}) {
	l.Error(args...)
	l.Flush()
}

// Fatalf prints formatted output to stderr and exits 1
func (l *Logger) Fatalf(format string, args ...interface{}) {
	l.Errorf(format, args...)
	l.Flush()
}

// WithPrefix allows setting of stdout and stderr prefixes and enables chaining
func (l *Logger) WithPrefix(prefix string) *Logger {
	l.stdout.SetPrefix(prefix)
	l.stderr.SetPrefix(prefix)
	return l
}

// Flush will call os.Exit if logger accumulated errors
func (l *Logger) Flush() {
	if l.errors > 0 {
		if l.errors > 125 {
			l.errors = 125
		}
		os.Exit(l.errors)
	}
}
