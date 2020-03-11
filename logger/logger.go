package logger

import (
	"fmt"
	"log"
	"os"

	"github.com/logrusorgru/aurora"
)

type Logger struct {
	debug  bool
	stdout *log.Logger
	stderr *log.Logger
	au     aurora.Aurora
}

// NewLogger returns *logger.Logger
func NewLogger(debug, noColor bool) *Logger {
	lg := Logger{
		debug:  debug,
		stdout: log.New(os.Stdout, "", 0),
		stderr: log.New(os.Stderr, "", 0),
		au:     aurora.NewAurora(!noColor), // flip noColor. --no-color -> noColor=true therefore colors=!noColor=false
	}
	return &lg
}

// Debug prints to stdout only if --debug is set
func (l *Logger) Debug(args ...interface{}) {
	if l.debug {
		l.Info(args...)
	}

}

// DebugF prints formatted text to stdout only if --debug is set
func (l *Logger) DebugF(format string, args ...interface{}) {
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

// Error prints to stderr
func (l *Logger) Error(args ...interface{}) {
	l.stderr.Println(l.au.Red(fmt.Sprint(args...)))
}

// Errorf prints to stderr
func (l *Logger) Errorf(format string, args ...interface{}) {
	l.stderr.Print(l.au.Red(fmt.Sprintf(format, args...)))
}

// Fatal prints to stderr and exits 1
func (l *Logger) Fatal(args ...interface{}) {
	l.Error(args...)
	os.Exit(1)
}

// Fatalf prints formatted output to stderr and exits 1
func (l *Logger) Fatalf(format string, args ...interface{}) {
	l.Errorf(format, args...)
	os.Exit(1)
}

// WithPrefix allows setting of stdout and stderr prefixes and enables chaining
func (l *Logger) WithPrefix(prefix string) *Logger {
	l.stdout.SetPrefix(prefix)
	l.stderr.SetPrefix(prefix)
	return l
}
