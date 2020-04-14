package logger

import (
	"fmt"
	"os"
	"sync"

	"github.com/logrusorgru/aurora"
)

// Logger is a sugar coating around log.Logger
type Logger struct {
	debug  bool
	errors int
	au     aurora.Aurora
}

var logger *Logger
var once sync.Once

// NewLogger returns *logger.Logger
func NewLogger(debug, noColor bool) *Logger {
	once.Do(func() {
		logger = &Logger{
			debug:  debug,
			errors: 0,
			au:     aurora.NewAurora(!noColor), // flip noColor. --no-color -> noColor=true therefore colors=!noColor=false
		}
	})
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
	fmt.Fprintln(os.Stdout, args...)
}

// Infof prints formatted text to stdout
func (l *Logger) Infof(format string, args ...interface{}) {
	fmt.Fprintf(os.Stdout, format, args...)
}

// Warn prints to stderr
func (l *Logger) Warn(args ...interface{}) {
	fmt.Fprintln(os.Stdout, l.au.Yellow(fmt.Sprint(args...)))
}

// Warnf prints to stderr
func (l *Logger) Warnf(format string, args ...interface{}) {
	fmt.Fprint(os.Stdout, l.au.Yellow(fmt.Sprintf(format, args...)))
}

// Error prints to stderr
func (l *Logger) Error(args ...interface{}) {
	l.errors++
	fmt.Fprintln(os.Stderr, l.au.Red(fmt.Sprint(args...)))
}

// Errorf prints to stderr
func (l *Logger) Errorf(format string, args ...interface{}) {
	l.errors++
	fmt.Fprint(os.Stderr, l.au.Red(fmt.Sprintf(format, args...)))
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

// Check prints a green check mark at the end of the current line
func (l *Logger) Check() {
	l.Infof("%s\n", l.au.Green("✓"))
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

// NumErrors returns the number of errors counted so far
func (l *Logger) NumErrors() int {
	return l.errors
}
