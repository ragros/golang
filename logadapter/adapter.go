package logadapter

import (
	"fmt"
	"log"
)

const (
	LevDEBUG = "[DEBUG]"
	LevVERBO = "[VERBO]"
	LevINFO  = "[INFO ]"
	LevWARN  = "[WARN ]"
	LevERROR = "[ERROR]"
	LevNOTE  = "[NOTE ]"
)

type ILogger interface {
	Outputln(depth int, lev, msg string)
}

type stdLogWarper struct {
	std *log.Logger
}

func (l *stdLogWarper) Outputln(depth int, lev, msg string) {
	l.std.Output(depth+1, lev+msg)
}

type logger struct {
	ILogger
}

func New(l ILogger) *logger {
	return &logger{l}
}

func NewFromStd(l *log.Logger) *logger {
	w := &stdLogWarper{l}
	return &logger{w}
}

func (l *logger) Debug(args ...interface{}) {
	l.Outputln(2, LevDEBUG, fmt.Sprint(args...))
}

func (l *logger) Verbose(args ...interface{}) {
	l.Outputln(2, LevVERBO, fmt.Sprint(args...))
}

func (l *logger) Info(args ...interface{}) {
	l.Outputln(2, LevINFO, fmt.Sprint(args...))
}

func (l *logger) Warn(args ...interface{}) {
	l.Outputln(2, LevWARN, fmt.Sprint(args...))
}

func (l *logger) Error(args ...interface{}) {
	l.Outputln(2, LevERROR, fmt.Sprint(args...))
}

func (l *logger) Note(args ...interface{}) {
	l.Outputln(2, LevNOTE, fmt.Sprint(args...))
}

func (l *logger) Debugf(format string, args ...interface{}) {
	l.Outputln(2, LevDEBUG, fmt.Sprintf(format, args...))
}

func (l *logger) Verbosef(format string, args ...interface{}) {
	l.Outputln(2, LevVERBO, fmt.Sprintf(format, args...))
}

func (l *logger) Infof(format string, args ...interface{}) {
	l.Outputln(2, LevINFO, fmt.Sprintf(format, args...))
}

func (l *logger) Warnf(format string, args ...interface{}) {
	l.Outputln(2, LevWARN, fmt.Sprintf(format, args...))
}

func (l *logger) Errorf(format string, args ...interface{}) {
	l.Outputln(2, LevERROR, fmt.Sprintf(format, args...))
}

func (l *logger) Notef(format string, args ...interface{}) {
	l.Outputln(2, LevNOTE, fmt.Sprintf(format, args...))
}
