# slog

slog is a micro log libray.log format is use default.
log use ConsolePrinter as default(least level) ,you can use it without any configuration.

in advance,use AddPrinter config you output mode.once operated the default
console printer is no longer exist.so add it if needed.

	-logmode=stdout:info,file:warn
	-logf_dir=./log
	-logf_name=app
	-logf_ksize=1024
	-logf_blockmillis=10
	-logf_bufferrow=1024
	-logf_backup=5

Attention: when init log by flags,you must call InitByFlags.
option level: debug verbose(verbo) info warn error note


## Example
```
func init() {
	flag.Set("logmode", "stdout:debug,file:warn")
	flag.Set("logf_ksize","200")
	slog.InitByFlags()
}
func main() {
	for i := 0; i < 10000; i++ {
		<-time.After(time.Nanosecond)
		slog.Debug("debug msg:", i)
		slog.Debugf("debugf msg %d", i)
		slog.Verbose("Verbose msg:", i)
		slog.Verbosef("Verbosef msg %d", i)
		slog.Info("Info msg:", i)
		slog.Infof("Infof msg %d", i)
		slog.Warn("Warn msg:", i)
		slog.Warnf("Warnf msg %d", i)
		slog.Error("Error msg:", i)
		slog.Errorf("Errorf msg %d", i)
		slog.Note("important info, not an error:", i)
	}
	slog.Exit("wait for flush to file.")
}
```

## Logger instance

user New() get a new Logger instance,support Print,Println,Printf,Output as std lib log.
```	
f, _ := slog.NewFilePrinter(1024, 9, "", "", 0, 0)
logger := slog.New(slog.LevINFO, f).AddPrinter(slog.LevINFO, slog.NewConsole())
logger.Print("hello")
```
