/*
slog is a micro log libray.log format is use default.
log use ConsolePrinter as default(at level DEBUG) ,you can use it without any configuration.

in advance,use cmdline args configure you output mode and other options.once operated the default
console printer is no longer exist.so add it if needed.

	-logmode=stdout:info,file:warn
	-logf_dir=.
	-logf_name=app
	-logf_ksize=10
	-logf_blockmillis=200
	-logf_bufferrow=123
	-logf_backup=5

or you can set up by code:
	flag.Set("logmode", "stdout:error,file:debug")
	//flag.Set("logmode", "stdout:verbo,file:fatal")
	slog.InitByFlags()

Attention: anywhere you must call InitByFlags.(we support setup by code which made me don't known when flags is ready ).
And slog's file output use one goroutine,it not worth to support flush file by hand which means a lock is needed.
if you want guarantee all log flush to file before exit,I suggest just wait for a few second .
*/
package slog

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/ragros/golang/logadapter"
)

type flags struct {
	out              string
	file_dir         string
	file_name        string
	file_ksize       int
	file_blockmillis int
	file_bufferrow   int
	file_backup      int

	udp_bufferrow   int
	udp_blockmillis int
	udp_addr        string
}

type lev int

const (
	LevDEBUG   lev = 1
	LevVERBOSE lev = 2
	LevINFO    lev = 4
	LevWARN    lev = 8
	LevERROR   lev = 0x10
	LevNOTE    lev = 0x20
	levFATAL   lev = 0x40
)

var (
	_flags        flags
	preset        *logger = New(LevDEBUG, NewConsole())
	adapterTagMap         = map[int]string{
		logadapter.LevDEBUG: "[DEBUG]",
		logadapter.LevVERBO: "[VERBO]",
		logadapter.LevINFO:  "[INFO ]",
		logadapter.LevWARN:  "[WARN ]",
		logadapter.LevERROR: "[ERROR]",
		logadapter.LevNOTE:  "[NOTE ]",
	}
)

func init() {
	flag.StringVar(&_flags.out, "logmode", "stdout:debug", "out mode,like this: stdout:info,file:warn,udp:warn")
	flag.StringVar(&_flags.file_dir, "logf_dir", ".", "log file dir")
	flag.StringVar(&_flags.file_name, "logf_name", "", "log file name")
	flag.IntVar(&_flags.file_ksize, "logf_ksize", 4*1024, "log file max size ,kB unit")
	flag.IntVar(&_flags.file_blockmillis, "logf_blockmillis", 1000, "file loging blocked working thread if output is busy")
	flag.IntVar(&_flags.file_bufferrow, "logf_bufferrow", (1024 * 24), "file logger cached row number without blocking if output is busy")
	flag.IntVar(&_flags.file_backup, "logf_backup", 9, "log file max backup count")
	flag.StringVar(&_flags.udp_addr, "logu_addr", "localhost:12345", "udp log addr")
	flag.IntVar(&_flags.udp_blockmillis, "logu_blockmillis", 50, "udp loging blocked working thread if output is busy")
	flag.IntVar(&_flags.udp_bufferrow, "logu_bufferrow", (1024 * 24), "udp logger cached row number without blocking if output is busy")
}

/*
	calling when configurate by flags
*/
func InitByFlags() {
	if !flag.Parsed() {
		flag.Parse()
	}
	alp := _flags.parse()
	if len(alp) == 0 {
		return
	}
	preset.Reset()
	for _, p := range alp {
		preset.AddPrinter(p.l, p.p)
	}
}

func Default() *logger {
	return preset
}

func (this *flags) parse() []*levPrinter {
	defer func() {
		if e := recover(); e != nil {
			fmt.Fprintf(os.Stderr, "parse slog flag failed: %v", e)
			os.Exit(0)
		}
	}()

	this.out = strings.ToUpper(this.out)
	am := strings.Split(this.out, ",")
	l := make([]*levPrinter, 0)
	for _, m := range am {
		ml := strings.Split(m, ":")
		if ml[0] == "FILE" {
			p, err := NewFilePrinter(this.file_ksize, this.file_backup,
				this.file_dir, this.file_name,
				this.file_blockmillis, this.file_bufferrow)
			if err != nil {
				panic(err)
			}
			l = append(l, &levPrinter{stringLev(ml[1]), p})
		} else if ml[0] == "STDOUT" {
			l = append(l, &levPrinter{stringLev(ml[1]), &Console{}})
		} else if ml[0] == "UDP" {
			p, err := NewUdpPrinter(this.udp_addr, this.udp_bufferrow, this.udp_blockmillis)
			if err != nil {
				panic(err)
			}
			l = append(l, &levPrinter{stringLev(ml[1]), p})
		}
	}
	return l
}

func (this *lev) String() string {
	switch *this {
	case LevDEBUG:
		return "[DEBUG]"
	case LevVERBOSE:
		return "[VERBO]"
	case LevINFO:
		return "[INFO ]"
	case LevWARN:
		return "[WARN ]"
	case LevERROR:
		return "[ERROR]"
	case LevNOTE:
		return "[NOTE ]"
	default:
		return "[FATAL]"
	}
}

func stringLev(l string) lev {
	l = strings.ToUpper(l)
	switch l {
	case "VERBO", "VERBOSE":
		return LevVERBOSE
	case "INFO":
		return LevINFO
	case "WARN", "WARNING":
		return LevWARN
	case "ERROR":
		return LevERROR
	case "NOTE":
		return LevNOTE
	default:
		return LevDEBUG
	}
}

type logger struct {
	rootLev lev
	f       Formater
	lp      []*levPrinter
}

func New(lv lev, p Printer) *logger {
	l := &logger{
		rootLev: lv,
		f:       &DefaultFormater{},
		lp:      []*levPrinter{&levPrinter{lv, p}},
	}
	return l
}

func (l *logger) AddPrinter(lv lev, p Printer) *logger {
	if lv < l.rootLev {
		l.rootLev = lv
	}
	l.lp = append(l.lp, &levPrinter{lv, p})
	return l
}

func (l *logger) SetFormater(f Formater) *logger {
	l.f = f
	return l
}

func (l *logger) Reset() *logger {
	l.lp = nil
	l.rootLev = LevNOTE
	l.f = &DefaultFormater{}
	return l
}

func (l *logger) Debug(args ...interface{}) {
	l.out(LevDEBUG, args...)
}
func (l *logger) Verbose(args ...interface{}) {
	l.out(LevVERBOSE, args...)
}
func (l *logger) Info(args ...interface{}) {
	l.out(LevINFO, args...)
}
func (l *logger) Warn(args ...interface{}) {
	l.out(LevWARN, args...)
}
func (l *logger) Error(args ...interface{}) {
	l.out(LevERROR, args...)
}
func (l *logger) Note(args ...interface{}) {
	l.out(LevNOTE, args...)
}
func (l *logger) Fatal(args ...interface{}) {
	l.out(levFATAL, args...)
	l.printStack(true)
	<-time.After(2 * time.Second)
	os.Exit(2)
}
func (l *logger) Exit(args ...interface{}) {
	l.out(LevNOTE, args...)
	<-time.After(2 * time.Second)
	os.Exit(2)
}

func (l *logger) Debugf(format string, args ...interface{}) {
	l.outf(LevDEBUG, format, args...)
}
func (l *logger) Verbosef(format string, args ...interface{}) {
	l.outf(LevVERBOSE, format, args...)
}
func (l *logger) Infof(format string, args ...interface{}) {
	l.outf(LevINFO, format, args...)
}
func (l *logger) Warnf(format string, args ...interface{}) {
	l.outf(LevWARN, format, args...)
}
func (l *logger) Errorf(format string, args ...interface{}) {
	l.outf(LevERROR, format, args...)
}
func (l *logger) Notef(format string, args ...interface{}) {
	l.outf(LevNOTE, format, args...)
}
func (l *logger) Fatalf(format string, args ...interface{}) {
	l.outf(levFATAL, format, args...)
	l.printStack(true)
	<-time.After(2 * time.Second)
	os.Exit(2)
}

func (l *logger) Exitf(format string, args ...interface{}) {
	l.outf(LevNOTE, format, args...)
	<-time.After(2 * time.Second)
	os.Exit(2)
}

func Debug(args ...interface{}) {
	preset.out(LevDEBUG, args...)
}
func Verbose(args ...interface{}) {
	preset.out(LevVERBOSE, args...)
}
func Info(args ...interface{}) {
	preset.out(LevINFO, args...)
}
func Warn(args ...interface{}) {
	preset.out(LevWARN, args...)
}
func Error(args ...interface{}) {
	preset.out(LevERROR, args...)
}
func Note(args ...interface{}) {
	preset.out(LevNOTE, args...)
}
func Fatal(args ...interface{}) {
	preset.out(levFATAL, args...)
	preset.printStack(true)
	<-time.After(2 * time.Second)
	os.Exit(2)
}
func Exit(args ...interface{}) {
	preset.out(LevNOTE, args...)
	<-time.After(2 * time.Second)
	os.Exit(2)
}

func Debugf(format string, args ...interface{}) {
	preset.outf(LevDEBUG, format, args...)
}
func Verbosef(format string, args ...interface{}) {
	preset.outf(LevVERBOSE, format, args...)
}
func Infof(format string, args ...interface{}) {
	preset.outf(LevINFO, format, args...)
}
func Warnf(format string, args ...interface{}) {
	preset.outf(LevWARN, format, args...)
}
func Errorf(format string, args ...interface{}) {
	preset.outf(LevERROR, format, args...)
}
func Notef(format string, args ...interface{}) {
	preset.outf(LevNOTE, format, args...)
}
func Fatalf(format string, args ...interface{}) {
	preset.outf(levFATAL, format, args...)
	preset.printStack(true)
	<-time.After(2 * time.Second)
	os.Exit(2)
}
func Exitf(format string, args ...interface{}) {
	preset.outf(LevNOTE, format, args...)
	<-time.After(2 * time.Second)
	os.Exit(2)
}

/**
Print,Printf,Println,Output to adapt standard lib log
**/
func (l *logger) Print(args ...interface{}) {
	l.Outputln(2, logadapter.LevNOTE, fmt.Sprint(args...))
}
func (l *logger) Println(args ...interface{}) {
	l.Outputln(2, logadapter.LevNOTE, fmt.Sprint(args...))
}
func (l *logger) Printf(format string, args ...interface{}) {
	l.Outputln(2, logadapter.LevNOTE, fmt.Sprintf(format, args...))
}

func (l *logger) Outputln(calldepth, lev int, msg string) {
	s := l.f.Format(calldepth, adapterTagMap[lev], msg)
	ps := l.lp
	for _, n := range ps {
		if err := n.p.Print(s); err != nil {
			l.slogerr(err)
		}
	}

}

func (l *logger) out(lv lev, args ...interface{}) {
	if l.rootLev <= lv {
		s := l.f.Format(2, lv.String(), fmt.Sprint(args...))
		ps := l.lp
		for _, n := range ps {
			if lv >= n.l {
				if err := n.p.Print(s); err != nil {
					l.slogerr(err)
				}
			}
		}
	}
}

func (l *logger) outf(lv lev, format string, args ...interface{}) {
	if l.rootLev <= lv {
		s := l.f.Format(2, lv.String(), fmt.Sprintf(format, args...))
		ps := l.lp
		for _, n := range ps {
			if lv >= n.l {
				if err := n.p.Print(s); err != nil {
					l.slogerr(err)
				}
			}
		}
	}
}

func (l *logger) printStack(all bool) {
	n := 500
	if all {
		n = 1000
	}
	var trace []byte

	for i := 0; i < 5; i++ {
		n *= 2
		trace = make([]byte, n)
		nbytes := runtime.Stack(trace, all)
		if nbytes <= len(trace) {
			n = nbytes
			break
		}
	}
	ms := string(trace[:n])
	ps := l.lp
	for _, n := range ps {
		if err := n.p.Print(&ms); err != nil {
			l.slogerr(err)
		}
	}
}

type levPrinter struct {
	l lev
	p Printer
}

//when log system has error,we will try record as much as possible(unrealize)
func (l *logger) slogerr(err error) {
	fmt.Fprint(os.Stderr, err.Error())
}
