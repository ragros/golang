package slog

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Printer interface {
	Print(s *string) error
}

type Console struct {
}

func NewConsole() *Console {
	return &Console{}
}

func (this *Console) Print(s *string) error {
	_, err := fmt.Fprint(os.Stderr, *s)
	return err
}

type FilePrinter struct {
	ch          chan *string
	blockMillis time.Duration
	baseName    string
	file        *os.File
	rsize       int
	csize       int
	backup      int
}

// NewFilePrinter
//	sizeKB: 日志文件滚动大小，以KB为单位
//	rollCount:日志文件滚动个数
//	dir:   日志输出目录,""为当前
//	name:  日志文件名称,""为程序名
//	blockMillis:缓冲队列满时，日志线程阻塞工作线程最大毫秒数。越大则丢日志的可能性越低
//	buffer_rows:缓冲队列长度(记录条数)
func NewFilePrinter(sizeKB, rollCount int, dir, name string, blockMillis, bufferRows int) (Printer, error) {
	var e error
	if name == "" {
		name = filepath.Base(os.Args[0])
		if i := strings.LastIndex(name, "."); i != -1 {
			name = name[:i]
		}
	}
	if blockMillis < 10 {
		blockMillis = 10
	}
	if bufferRows < 1024 {
		bufferRows = 1024
	}
	if dir == "" {
		dir = "."
	}
	p := &FilePrinter{
		ch:          make(chan *string, bufferRows),
		rsize:       sizeKB * 1024,
		blockMillis: time.Millisecond * time.Duration(blockMillis),
		backup:      rollCount,
	}
	dir, e = filepath.Abs(dir)
	if e != nil {
		return nil, e
	}
	e = os.MkdirAll(dir, os.ModeDir)
	if e != nil {
		return nil, e
	}
	p.baseName = filepath.Clean(dir + string(filepath.Separator) + name + ".log")
	p.file, e = os.OpenFile(p.baseName, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0666)
	if e != nil {
		return nil, e
	}
	fs, _ := p.file.Stat()
	p.csize = int(fs.Size())
	t := time.Now()
	s := t.Format("[01-02 15:04:05.999][NOTE ] ------------ start ------------\n")
	p.Print(&s)
	go p.flush()
	return p, nil
}

func (this *FilePrinter) Print(s *string) error {
	select {
	case this.ch <- s:
	case <-time.After(this.blockMillis):
		return fmt.Errorf("enqueue timeout:%s", *s)
	}
	return nil
}

func (this *FilePrinter) checkfile() {
	if this.csize > this.rsize {
		this.file.Sync()
		this.file.Close()
		this.roll()
		this.file, _ = os.OpenFile(this.baseName, os.O_CREATE|os.O_WRONLY, 0666)
		this.csize = 0
	}
}

func (this *FilePrinter) roll() {
	if this.backup == 0 {
		os.Remove(this.baseName)
		return
	}
	os.Remove(this.baseName + "." + strconv.Itoa(this.backup))
	for i := this.backup - 1; i > 0; i-- {
		o := this.baseName + "." + strconv.Itoa(i)
		n := this.baseName + "." + strconv.Itoa(i+1)
		os.Rename(o, n)
	}
	os.Rename(this.baseName, this.baseName+".1")
}

func (this *FilePrinter) flush() {
	var n int
	var e error
	for {
		n = 0
	loop:
		for {
			select {
			case s := <-this.ch:
				this.checkfile()
				n, e = this.file.WriteString(*s)
				if e != nil {
					this.csize = this.rsize + 1
					break
				} else {
					this.csize += n
				}
			default:
				break loop
			}
		}
		if n != 0 {
			this.file.Sync()
		}
		<-time.After(time.Second)
	}
}

type UdpPrinter struct {
	ch          chan *string
	blockMillis time.Duration
	conn        *net.UDPConn
}

func NewUdpPrinter(addr string, maxrn int, blockMillis int) (Printer, error) {
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, err
	}
	if blockMillis < 10 {
		blockMillis = 10
	}
	p := &UdpPrinter{
		ch:          make(chan *string, maxrn),
		blockMillis: time.Millisecond * time.Duration(blockMillis),
	}
	p.conn, err = net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		return nil, err
	}
	t := time.Now()
	s := t.Format("[01-02 15:04:05.999][NOTE ] ------------ start ------------\n")
	p.Print(&s)
	go p.flush()
	return p, err

}

func (this *UdpPrinter) Print(s *string) error {
	select {
	case this.ch <- s:
	case <-time.After(this.blockMillis):
		return fmt.Errorf("enqueue timeout:%s", *s)
	}
	return nil
}

func (this *UdpPrinter) flush() {
	var bs []byte
	for v := range this.ch {
		bs = []byte(*v)
		this.conn.Write(bs)
	}
}
