package log

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"sync/atomic"
)

const (
	calldepth = 2
)

var (
	verbosity = VLevel(0)

	iLog = log.New(os.Stderr, "INFO ", log.Lshortfile)
	eLog = log.New(os.Stderr, "ERROR ", log.Lshortfile)
	wLog = log.New(os.Stderr, "WARN ", log.Lshortfile)
	fLog = log.New(os.Stderr, "FATAL ", log.Lshortfile)
)

func SetVerbosity(lvl int) {
	verbosity.set(int32(lvl))
}

type VLevel int32

func (l *VLevel) get() VLevel {
	return VLevel(atomic.LoadInt32((*int32)(l)))
}

func (l *VLevel) String() string {
	return strconv.FormatInt(int64(*l), 10)
}

func (l *VLevel) Get() interface{} {
	return l.get()
}

func (l *VLevel) Set(val string) error {
	vi, err := strconv.Atoi(val)
	if err != nil {
		return err
	}
	l.set(int32(vi))
	return nil
}

func (l *VLevel) set(lvl int32) {
	atomic.StoreInt32((*int32)(l), lvl)
}

type VLogger bool

func V(level VLevel) VLogger {
	return VLogger(verbosity.get() >= level)
}

func (vl VLogger) Info(v ...interface{}) {
	if vl {
		iLog.Output(calldepth, fmt.Sprint(v...))
	}
}

func (vl VLogger) Infof(format string, v ...interface{}) {
	if vl {
		iLog.Output(calldepth, fmt.Sprintf(format, v...))
	}
}

func Info(v ...interface{}) {
	iLog.Output(calldepth, fmt.Sprint(v...))
}

func Infof(format string, v ...interface{}) {
	iLog.Output(calldepth, fmt.Sprintf(format, v...))
}

func Error(v ...interface{}) {
	eLog.Output(calldepth, fmt.Sprint(v...))
}

func Errorf(format string, v ...interface{}) {
	eLog.Output(calldepth, fmt.Sprintf(format, v...))
}

func Warning(format string, v ...interface{}) {
	wLog.Output(calldepth, fmt.Sprint(v...))
}

func Warningf(format string, v ...interface{}) {
	wLog.Output(calldepth, fmt.Sprintf(format, v...))
}

func Fatal(v ...interface{}) {
	fLog.Output(calldepth, fmt.Sprint(v...))
	os.Exit(1)
}

func Fatalf(format string, v ...interface{}) {
	fLog.Output(calldepth, fmt.Sprintf(format, v...))
	os.Exit(1)
}
