package slog

import (
	"fmt"
	"runtime"
	"strings"
	"time"
)

type _Formater interface {
	Format(lev lev, args ...interface{}) *string
	Formatf(lev lev, format string, args ...interface{}) *string
}

type _DefaultFormater struct {
}

func (this *_DefaultFormater) Format(lev lev, args ...interface{}) *string {
	s := fmt.Sprintln(args...)
	msg := this.combine(lev, &s)
	return msg
}

func (this *_DefaultFormater) Formatf(lev lev, format string, args ...interface{}) *string {
	if format[len(format)-1] == '\n' {
		format = format[:len(format)-1]
	}
	s := fmt.Sprintf(format, args...)
	msg := this.combine(lev, &s)
	return msg
}

func (this *_DefaultFormater) combine(lev lev, msg *string) *string {
	t := time.Now()
	_, m, d := t.Date()
	_, file, line, ok := runtime.Caller(4)
	if !ok {
		file = "???"
		line = 0
	} else {
		slash := strings.LastIndex(file, "/")
		if slash >= 0 {
			slash = strings.LastIndex(file[:slash], "/")
			if slash >= 0 {
				file = file[slash+1:]
			}
		}
	}
	ml := len(*msg)
	if ml > 0 && (*msg)[ml-1] == '\n' {
		ml -= 1
	}
	h := fmt.Sprintf("[%02d-%02d %02d:%02d:%02d.%03d][%-5s] %s (%s:%d)\n",
		m, d, t.Hour(), t.Minute(), t.Second(), t.Nanosecond()/1000000,
		lev.String(), (*msg)[:ml], file, line)
	return &h
}
