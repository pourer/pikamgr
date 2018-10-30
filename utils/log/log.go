package log

import (
	"fmt"
	"io"
	"math"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"
)

const (
	Ldate         = 1 << iota                     // the date in the local time zone: 2009/01/23
	Ltime                                         // the time in the local time zone: 01:23:23
	Lmicroseconds                                 // microsecond resolution: 01:23:23.123123.  assumes Ltime.
	Llongfile                                     // full file name and line number: /a/b/c/d.go:23
	Lshortfile                                    // final file name element and line number: d.go:23. overrides Llongfile
	LUTC                                          // if Ldate or Ltime is set, use UTC rather than the local time zone
	LstdFlags     = Ldate | Ltime | Lmicroseconds // initial values for the standard logger
)

const (
	Loff   = math.MaxInt32 // off level: disable log
	Lfatal = 70000         // fatal level, call os.Exit(-1): [FATAL]
	Lpanic = 60000         // panic level, call panic(): [PANIC]
	Lerror = 50000         // error level: [ERROR]
	Lwarn  = 40000         // warning level: [WARN]
	Linfo  = 30000         // info level: [INFO]
	Ldebug = 20000         // debug level: [DEBUG]
	Ltrace = 10000         // trace level: [TRACE]

	LstdLevel = Ltrace // default level: Ltrace
)

var std = New(os.Stderr, LstdLevel, LstdFlags|Llongfile)

// A Logger represents an active logging object that generates lines of
// output to an io.Writer.  Each logging operation makes a single call to
// the Writer's Write method.  A Logger can be used simultaneously from
// multiple goroutines; it guarantees to serialize access to the Writer.
type Logger struct {
	level int
	mu    sync.Mutex // ensure atomic writes; protect the following fields
	out   io.Writer
	flag  int
	buf   []byte
}

// New creates a new Logger.   The out variable sets the
// destination to which log data will be written.
// The level argument defines the logging levels.
// The flag argument defines the logging properties.
func New(out io.Writer, level int, flag int) *Logger {
	return &Logger{
		out:   out,
		level: level,
		flag:  flag,
	}
}

// SetOutput sets the output destination for the logger.
func (l *Logger) SetOutput(out io.Writer) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.out = out
}

// Level returns the output levels for the logger.
func (l *Logger) Level() int {
	return l.level
}

// SetLevel sets the output levels for the logger.
func (l *Logger) SetLevel(level int) {
	l.level = level
}

// Flags returns the output flags for the logger
func (l *Logger) Flags() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.flag
}

// SetFlags sets the output flags for the logger.
func (l *Logger) SetFlags(flag int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.flag = flag
}

func itoa(buf *[]byte, i int, wid int) {
	// Assemble decimal in reverse order.
	var b [20]byte
	bp := len(b) - 1
	for i >= 10 || wid > 1 {
		wid--
		q := i / 10
		b[bp] = byte('0' + i - q*10)
		bp--
		i = q
	}
	// i < 10
	b[bp] = byte('0' + i)
	*buf = append(*buf, b[bp:]...)
}

func LevelToString(level int) string {
	switch level {
	case Loff:
		return "[OFF]"
	case Lfatal:
		return "[FATAL]"
	case Lpanic:
		return "[PANIC]"
	case Lerror:
		return "[ERROR]"
	case Lwarn:
		return "[WARN]"
	case Linfo:
		return "[INFO]"
	case Ldebug:
		return "[DEBUG]"
	case Ltrace:
		return "[TRACE]"
	default:
		return ""
	}
}

func StringToLevel(s string) int {
	switch strings.ToLower(s) {
	case "off":
		return Loff
	case "fatal":
		return Lfatal
	case "panic":
		return Lpanic
	case "error":
		return Lerror
	case "warn":
		return Lwarn
	case "info":
		return Linfo
	case "debug":
		return Ldebug
	case "trace":
		return Ltrace
	default:
		return LstdLevel
	}
}

func (l *Logger) formatHeader(buf *[]byte, t time.Time, file string, line int, level int) {
	if l.flag&LUTC != 0 {
		t = t.UTC()
	}

	if l.flag&(Ldate|Ltime|Lmicroseconds) != 0 {
		if l.flag&Ldate != 0 {
			year, month, day := t.Date()
			itoa(buf, year, 4)
			*buf = append(*buf, '/')
			itoa(buf, int(month), 2)
			*buf = append(*buf, '/')
			itoa(buf, day, 2)
			*buf = append(*buf, ' ')
		}

		if l.flag&(Ltime|Lmicroseconds) != 0 {
			hour, min, sec := t.Clock()
			itoa(buf, hour, 2)
			*buf = append(*buf, ':')
			itoa(buf, min, 2)
			*buf = append(*buf, ':')
			itoa(buf, sec, 2)
			if l.flag&Lmicroseconds != 0 {
				*buf = append(*buf, '.')
				itoa(buf, t.Nanosecond()/1e3, 6)
			}
			*buf = append(*buf, ' ')
		}
	}

	*buf = append(*buf, LevelToString(level)...)
	*buf = append(*buf, ' ')

	if l.flag&(Lshortfile|Llongfile) != 0 {
		if l.flag&Lshortfile != 0 {
			short := file
			for i := len(file) - 1; i > 0; i-- {
				if file[i] == '/' {
					short = file[i+1:]
					break
				}
			}
			file = short
		}

		*buf = append(*buf, ' ')
		*buf = append(*buf, file...)
		*buf = append(*buf, ':')
		itoa(buf, line, -1)
		*buf = append(*buf, ": "...)
	}
}

func (l *Logger) Outputf(level int, callDepth int, format string, args ...interface{}) error {
	if level < l.level {
		return nil
	}
	return l.output(level, callDepth+1, fmt.Sprintf(format, args...))
}

func (l *Logger) Outputln(level int, callDepth int, v ...interface{}) error {
	if level < l.level {
		return nil
	}
	return l.output(level, callDepth+1, fmt.Sprintln(v...))
}

func (l *Logger) Output(level int, callDepth int, v ...interface{}) error {
	if level < l.level {
		return nil
	}
	return l.output(level, callDepth+1, fmt.Sprint(v...))
}

// Output writes the output for a logging event.  The string s contains
// the text to print after the prefix specified by the flags of the
// Logger.  A newline is appended if the last character of s is not
// already a newline.  Calldepth is used to recover the PC and is
// provided for generality, although at the moment on all pre-defined
// paths it will be 2.
func (l *Logger) output(level int, callDepth int, s string) error {
	now := time.Now() // get this early.
	var file string
	var line int
	if l.flag&(Lshortfile|Llongfile) != 0 {
		var ok bool
		_, file, line, ok = runtime.Caller(callDepth)
		if !ok {
			file = "???"
			line = 0
		}
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	l.buf = l.buf[:0]
	l.formatHeader(&l.buf, now, file, line, level)
	l.buf = append(l.buf, s...)
	if len(s) == 0 || s[len(s)-1] != '\n' {
		l.buf = append(l.buf, '\n')
	}
	_, err := l.out.Write(l.buf)
	return err
}

// Info calls l.Output to print to the logger.
// Arguments are handled in the manner of fmt.Println.
func (l *Logger) Info(v ...interface{}) {
	l.Output(Linfo, 2, v...)
}

func (l *Logger) Infoln(v ...interface{}) {
	l.Outputln(Linfo, 2, v...)
}

// Infof calls l.Output to print to the logger.
// Arguments are handled in the manner of fmt.Printf.
func (l *Logger) Infof(format string, v ...interface{}) {
	l.Outputf(Linfo, 2, format, v...)
}

// Error calls l.Output to print to the logger.
// Arguments are handled in the manner of fmt.Println.
func (l *Logger) Error(v ...interface{}) {
	l.Output(Lerror, 2, v...)
}

func (l *Logger) Errorln(v ...interface{}) {
	l.Outputln(Lerror, 2, v...)
}

// Errorf calls l.Output to print to the logger.
// Arguments are handled in the manner of fmt.Printf.
func (l *Logger) Errorf(format string, v ...interface{}) {
	l.Outputf(Lerror, 2, format, v...)
}

// Debug calls l.Output to print to the logger.
// Arguments are handled in the manner of fmt.Println.
func (l *Logger) Debug(v ...interface{}) {
	l.Output(Ldebug, 2, v...)
}

func (l *Logger) Debugln(v ...interface{}) {
	l.Outputln(Ldebug, 2, v...)
}

// Debugf calls l.Output to print to the logger.
// Arguments are handled in the manner of fmt.Printf.
func (l *Logger) Debugf(format string, v ...interface{}) {
	l.Outputf(Ldebug, 2, format, v...)
}

// Trace calls l.Output to print to the logger.
// Arguments are handled in the manner of fmt.Println.
func (l *Logger) Trace(v ...interface{}) {
	l.Output(Ltrace, 2, v...)
}

func (l *Logger) Traceln(v ...interface{}) {
	l.Outputln(Ltrace, 2, v...)
}

// Trace calls l.Output to print to the logger.
// Arguments are handled in the manner of fmt.Printf.
func (l *Logger) Tracef(format string, v ...interface{}) {
	l.Outputf(Ltrace, 2, format, v...)
}

// Warn calls l.Output to print to the logger.
// Arguments are handled in the manner of fmt.Println.
func (l *Logger) Warn(v ...interface{}) {
	l.Output(Lwarn, 2, v...)
}

func (l *Logger) Warnln(v ...interface{}) {
	l.Outputln(Lwarn, 2, v...)
}

// Warnf calls l.Output to print to the logger.
// Arguments are handled in the manner of fmt.Printf.
func (l *Logger) Warnf(format string, v ...interface{}) {
	l.Outputf(Lwarn, 2, format, v...)
}

// Fatal calls l.Output to print to the logger and is followed by a call to os.Exit(-1)
// Arguments are handled in the manner of fmt.Println
func (l *Logger) Fatal(v ...interface{}) {
	if Lfatal >= l.level {
		l.Output(Lfatal, 2, v...)
		os.Exit(1)
	}
}

func (l *Logger) Fatalln(v ...interface{}) {
	if Lfatal >= l.level {
		l.Outputln(Lfatal, 2, v...)
		os.Exit(1)
	}
}

// Fatal calls l.Output to print to the logger and is followed by a call to os.Exit(-1)
// Arguments are handled in the manner of fmt.Printf
func (l *Logger) Fatalf(format string, v ...interface{}) {
	if Lfatal >= l.level {
		l.Outputf(Lfatal, 2, format, v...)
		os.Exit(1)
	}
}

// Panic calls l.Output to print to the logger and is followed by a call to panic().
// Arguments are handled in the manner of fmt.Println.
func (l *Logger) Panic(v ...interface{}) {
	if Lpanic >= l.level {
		s := fmt.Sprint(v...)
		l.output(Lpanic, 2, s)
		panic(s)
	}
}

// Panicf calls l.Output to print to the logger and is followed by a call to panic().
// Arguments are handled in the manner of fmt.Printf.
func (l *Logger) Panicf(format string, v ...interface{}) {
	if Lpanic >= l.level {
		s := fmt.Sprintf(format, v...)
		l.output(Lpanic, 2, s)
		panic(s)
	}
}

func (l *Logger) Panicln(v ...interface{}) {
	if Lpanic >= l.level {
		s := fmt.Sprintln(v...)
		l.output(Lpanic, 2, s)
		panic(s)
	}
}

// SetOutput sets the output destination for the standard logger.
func SetOutput(w io.Writer) {
	std.mu.Lock()
	defer std.mu.Unlock()
	std.out = w
}

// Levels returns the output levels for the standard logger.
func Level() int {
	return std.Level()
}

// SetLevels sets the output levels for the standard logger.
func SetLevel(level int) {
	std.SetLevel(level)
}

// Flags returns the output flags for the standard logger.
func Flags() int {
	return std.Flags()
}

// SetFlags sets the output flags for the standard logger.
func SetFlags(flag int) {
	std.SetFlags(flag)
}

// These functions write to the standard logger.

// Info calls Output to print to the standard logger.
// Arguments are handled in the manner of fmt.Println.
func Info(v ...interface{}) {
	std.Output(Linfo, 2, v...)
}

// Infof calls Output to print to the standard logger.
// Arguments are handled in the manner of fmt.Printf.
func Infof(format string, v ...interface{}) {
	std.Outputf(Linfo, 2, format, v...)
}

func Infoln(v ...interface{}) {
	std.Outputln(Linfo, 2, v...)
}

// Error calls l.Output to print to the standard logger.
// Arguments are handled in the manner of fmt.Println.
func Error(v ...interface{}) {
	std.Output(Lerror, 2, v...)
}

// Errorf calls l.Output to print to the standard logger.
// Arguments are handled in the manner of fmt.Printf.
func Errorf(format string, v ...interface{}) {
	std.Outputf(Lerror, 2, format, v...)
}

func Errorln(v ...interface{}) {
	std.Outputln(Lerror, 2, v...)
}

// Debug calls l.Output to print to the standard logger.
// Arguments are handled in the manner of fmt.Println.
func Debug(v ...interface{}) {
	std.Output(Ldebug, 2, v...)
}

// Debugf calls l.Output to print to the standard logger.
// Arguments are handled in the manner of fmt.Printf.
func Debugf(format string, v ...interface{}) {
	std.Outputf(Ldebug, 2, format, v...)
}

func Debugln(v ...interface{}) {
	std.Outputln(Ldebug, 2, v...)
}

// Trace calls l.Output to print to the standard logger.
// Arguments are handled in the manner of fmt.Println.
func Trace(v ...interface{}) {
	std.Output(Ltrace, 2, v...)
}

// Tracef calls l.Output to print to the standard logger.
// Arguments are handled in the manner of fmt.Printf.
func Tracef(format string, v ...interface{}) {
	std.Outputf(Ltrace, 2, format, v...)
}

func Traceln(v ...interface{}) {
	std.Outputln(Ltrace, 2, v...)
}

// Warn calls l.Output to print to the standard logger.
// Arguments are handled in the manner of fmt.Println.
func Warn(v ...interface{}) {
	std.Output(Lwarn, 2, v...)
}

func Warnln(v ...interface{}) {
	std.Outputln(Lwarn, 2, v...)
}

func Warnf(format string, v ...interface{}) {
	std.Outputf(Lwarn, 2, format, v...)
}

// Fatal calls l.Output to print to the standard logger and is followed by a call to os.Exit(-1)
// Arguments are handled in the manner of fmt.Println
func Fatal(v ...interface{}) {
	if Lfatal >= std.level {
		std.Output(Lfatal, 2, v...)
		os.Exit(1)
	}
}

func Fatalln(v ...interface{}) {
	if Lfatal >= std.level {
		std.Outputln(Lfatal, 2, v...)
		os.Exit(1)
	}
}

// Fatalf calls l.Output to print to the standard logger and is followed by a call to os.Exit(-1)
// Arguments are handled in the manner of fmt.Printf
func Fatalf(format string, v ...interface{}) {
	if Lfatal >= std.level {
		std.Outputf(Lfatal, 2, format, v...)
		os.Exit(1)
	}
}

// Panic calls l.Output to print to the standard logger and is followed by a call to panic().
// Arguments are handled in the manner of fmt.Println.
func Panic(v ...interface{}) {
	if Lpanic >= std.level {
		s := fmt.Sprint(v...)
		std.output(Lpanic, 2, s)
		panic(s)
	}
}

func Panicln(v ...interface{}) {
	if Lpanic >= std.level {
		s := fmt.Sprintln(v...)
		std.output(Lpanic, 2, s)
		panic(s)
	}
}

// Panicf calls l.Output to print to the standard logger and is followed by a call to panic().
// Arguments are handled in the manner of fmt.Printf.
func Panicf(format string, v ...interface{}) {
	if Lpanic >= std.level {
		s := fmt.Sprintln(v...)
		std.output(Lpanic, 2, s)
		panic(s)
	}
}

// Output writes the output for a logging event.  The string s contains
// the text to print after the prefix specified by the flags of the
// Logger.  A newline is appended if the last character of s is not
// already a newline.  Calldepth is the count of the number of
// frames to skip when computing the file name and line number
// if Llongfile or Lshortfile is set; a value of 1 will print the details
// for the caller of Output.
func Output(level int, calldepth int, s string) error {
	if level < std.level {
		return nil
	}
	return std.output(level, calldepth+1, s) // +1 for this frame.
}
