/*
slog is a micro log libray.log format is use default.
log use ConsolePrinter as default(least level) ,you can use it without any configuration.

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
	"path/filepath"
	"runtime"
	"strings"
	"time"
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
	LevFATAL   lev = 0x40
)

var (
	_flags  flags
	rootLev lev
	errch   chan *string = make(chan *string)
	f       _Formater    = new(_DefaultFormater)
	lp      []*levPrinter
	//mux     sync.RWMutex
	//config  bool
	Logger *logger = &logger{}
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
	lp = []*levPrinter{
		&levPrinter{LevDEBUG, &_ConsolePrinter{}},
	}
	rootLev = LevDEBUG
	//go flusherr()

}

/*
	calling when configurate by flags
*/
func InitByFlags() {
	if !flag.Parsed() {
		flag.Parse()
	}
	alp := _flags.parse()
	if len(alp) != 0 {
		lp = alp
		rootLev = LevNOTE
	}
	for _, p := range alp {
		if p.l < rootLev {
			rootLev = p.l
		}
	}

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
			p, err := _NewFilePrinter(this.file_bufferrow, this.file_dir, this.file_name,
				this.file_ksize, this.file_backup, this.file_blockmillis)
			if err != nil {
				panic(err)
			}
			l = append(l, &levPrinter{stringLev(ml[1]), p})
		} else if ml[0] == "STDOUT" {
			l = append(l, &levPrinter{stringLev(ml[1]), &_ConsolePrinter{}})
		} else if ml[0] == "UDP" {
			p, err := _NewUdpNetPrinter(this.udp_addr, this.udp_bufferrow, this.udp_blockmillis)
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
		return "DEBUG"
	case LevVERBOSE:
		return "VERBO"
	case LevINFO:
		return "INFO"
	case LevWARN:
		return "WARN"
	case LevERROR:
		return "ERROR"
	case LevNOTE:
		return "NOTE"
	default:
		return "FATAL"
	}
}

func stringLev(l string) lev {
	l = strings.ToUpper(l)
	switch l {
	case "VERBO", "VERBOSE":
		return LevVERBOSE
	case "INFO":
		return LevINFO
	case "WARN":
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
}

func (*logger) Debug(args ...interface{}) {
	out(LevDEBUG, args...)
}
func (*logger) Verbose(args ...interface{}) {
	out(LevVERBOSE, args...)
}
func (*logger) Info(args ...interface{}) {
	out(LevINFO, args...)
}
func (*logger) Warn(args ...interface{}) {
	out(LevWARN, args...)
}
func (*logger) Error(args ...interface{}) {
	out(LevERROR, args...)
}
func (*logger) Note(args ...interface{}) {
	out(LevNOTE, args...)
}
func (*logger) Fatal(args ...interface{}) {
	out(LevFATAL, args...)
	printStack(true)
	<-time.After(2 * time.Second)
	os.Exit(2)
}
func (*logger) Exit(args ...interface{}) {
	out(LevNOTE, args...)
	<-time.After(2 * time.Second)
	os.Exit(2)
}

func (*logger) Debugf(format string, args ...interface{}) {
	outf(LevDEBUG, format, args...)
}
func (*logger) Verbosef(format string, args ...interface{}) {
	outf(LevVERBOSE, format, args...)
}
func (*logger) Infof(format string, args ...interface{}) {
	outf(LevINFO, format, args...)
}
func (*logger) Warnf(format string, args ...interface{}) {
	outf(LevWARN, format, args...)
}
func (*logger) Errorf(format string, args ...interface{}) {
	outf(LevERROR, format, args...)
}
func (*logger) Notef(format string, args ...interface{}) {
	outf(LevNOTE, format, args...)
}
func (*logger) Fatalf(format string, args ...interface{}) {
	outf(LevFATAL, format, args...)
	printStack(true)
	<-time.After(2 * time.Second)
	os.Exit(2)
}
func (*logger) Exitf(format string, args ...interface{}) {
	outf(LevNOTE, format, args...)
	<-time.After(2 * time.Second)
	os.Exit(2)
}

func Debug(args ...interface{}) {
	out(LevDEBUG, args...)
}
func Verbose(args ...interface{}) {
	out(LevVERBOSE, args...)
}
func Info(args ...interface{}) {
	out(LevINFO, args...)
}
func Warn(args ...interface{}) {
	out(LevWARN, args...)
}
func Error(args ...interface{}) {
	out(LevERROR, args...)
}
func Note(args ...interface{}) {
	out(LevNOTE, args...)
}
func Fatal(args ...interface{}) {
	out(LevFATAL, args...)
	printStack(true)
	<-time.After(2 * time.Second)

	os.Exit(2)
}
func Exit(args ...interface{}) {
	out(LevNOTE, args...)
	<-time.After(2 * time.Second)
	os.Exit(2)
}

func Debugf(format string, args ...interface{}) {
	outf(LevDEBUG, format, args...)
}
func Verbosef(format string, args ...interface{}) {
	outf(LevVERBOSE, format, args...)
}
func Infof(format string, args ...interface{}) {
	outf(LevINFO, format, args...)
}
func Warnf(format string, args ...interface{}) {
	outf(LevWARN, format, args...)
}
func Errorf(format string, args ...interface{}) {
	outf(LevERROR, format, args...)
}
func Notef(format string, args ...interface{}) {
	outf(LevNOTE, format, args...)
}
func Fatalf(format string, args ...interface{}) {
	outf(LevFATAL, format, args...)
	printStack(true)
	<-time.After(2 * time.Second)
	os.Exit(2)
}
func Exitf(format string, args ...interface{}) {
	outf(LevNOTE, format, args...)
	<-time.After(2 * time.Second)
	os.Exit(2)
}

func out(lv lev, args ...interface{}) {
	if rootLev <= lv {
		s := f.Format(lv, args...)
		for _, n := range lp {
			if lv >= n.l {
				n.p.Print(s)
			}
		}
	}
}

func outf(lv lev, format string, args ...interface{}) {
	if rootLev <= lv {
		s := f.Formatf(lv, format, args...)
		for _, n := range lp {
			if lv >= n.l {
				n.p.Print(s)
			}
		}
	}
}

func printStack(all bool) {
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
	for _, n := range lp {
		n.p.Print(&ms)
	}
}

type levPrinter struct {
	l lev
	p _Printer
}

//when log system has error,we will try record as much as possible(unrealize)
func slogerr(s *string) {
	select {
	case errch <- s:
	default:
		fmt.Fprint(os.Stderr, "slogerr timeout:"+*s)
	}
}
func flusherr() {
	for s := range errch {
		fmt.Fprint(os.Stderr, *s)
	}
}

func WriteFile(name string, format string, args ...interface{}) {
	s := fmt.Sprintf(format, args...)
	l := len(s)
	if l > 0 && s[l-1] == '\n' {
		s = s[0 : l-1]
	}
	f, err := os.OpenFile(filepath.Join(_flags.file_dir, name+".log"), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		return
	}
	defer f.Close()
	f.WriteString(fmt.Sprintf("[%s]%s\n", time.Now().Format("01-02 15:04:05.999"), s))
}
