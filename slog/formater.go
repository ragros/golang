package slog

import (
	"fmt"
	"runtime"
	"strings"
	"time"
)

type Formater interface {
	//calldepth Format==0
	Format(calldepth int, tag, msg string) *string
}

type DefaultFormater struct {
}

func (this *DefaultFormater) Format(calldepth int, tag, msg string) *string {
	t := time.Now()
	_, m, d := t.Date()
	_, file, line, ok := runtime.Caller(calldepth + 1)
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
	ml := len(msg)
	if ml > 0 && msg[ml-1] == '\n' {
		ml -= 1
	}
	h := fmt.Sprintf("[%02d-%02d %02d:%02d:%02d.%03d]%s%s(%s:%d)\n",
		m, d, t.Hour(), t.Minute(), t.Second(), t.Nanosecond()/1000000,
		tag, msg[:ml], file, line)
	return &h
}
