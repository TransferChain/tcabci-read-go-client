package tcabcireadgoclient

import (
	"context"
	"os"
	"runtime"

	log "github.com/sirupsen/logrus"
)

type Logger interface {
	Info(args ...interface{})
	Error(args ...interface{})
	Print(args ...interface{})
}

type logger struct {
	ctx        context.Context
	l          *log.Logger
	e          *log.Entry
	withSentry bool
	sentryDSN  string
}

func NewLogger(ctx context.Context) Logger {
	l := &logger{ctx: ctx}
	l.init()

	return l
}

func (l *logger) init() {
	l1 := log.New()
	l1.SetFormatter(&log.JSONFormatter{})
	l1.SetOutput(os.Stdout)

	l.l = l1
	l.e = l1.WithContext(l.ctx).WithFields(log.Fields{
		"name": "tcabci-read-go-client",
	})
}

func (l *logger) Print(args ...interface{}) {
	_, fn, no, _ := runtime.Caller(1)
	l.e.WithFields(log.Fields{
		"file": fn,
		"line": no,
	}).Print(args...)
}

func (l *logger) Info(args ...interface{}) {
	_, fn, no, _ := runtime.Caller(1)
	l.e.WithFields(log.Fields{
		"file": fn,
		"line": no,
	}).Info(args...)
}
func (l *logger) Error(args ...interface{}) {
	_, fn, no, _ := runtime.Caller(1)
	l.e.WithFields(log.Fields{
		"file": fn,
		"line": no,
	}).Error(args...)
}
