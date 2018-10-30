package log

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"strings"
	"testing"
	"time"
)

const (
	Rdate         = `[0-9][0-9][0-9][0-9]/[0-9][0-9]/[0-9][0-9]`
	Rtime         = `[0-9][0-9]:[0-9][0-9]:[0-9][0-9]`
	Rmicroseconds = `\.[0-9][0-9][0-9][0-9][0-9][0-9]`
	Rline         = `(57|59):` // must update if the calls to l.Printf / l.Print below move
	Rlongfile     = `.*/[A-Za-z0-9_\-]+\.go:` + Rline
	Rshortfile    = `[A-Za-z0-9_\-]+\.go:` + Rline
)

type tester struct {
	flag    int
	prefix  string
	pattern string // regexp that log output must match; we add ^ and expected_text$ always
}

var tests = []tester{
	// individual pieces:
	{0, "", ""},
	{0, "XXX", "XXX"},
	{Ldate, "", Rdate + " "},
	{Ltime, "", Rtime + " "},
	{Ltime | Lmicroseconds, "", Rtime + Rmicroseconds + " "},
	{Lmicroseconds, "", Rtime + Rmicroseconds + " "}, // microsec implies time
	{Llongfile, "", Rlongfile + " "},
	{Lshortfile, "", Rshortfile + " "},
	{Llongfile | Lshortfile, "", Rshortfile + " "}, // shortfile overrides longfile
	// everything at once:
	{Ldate | Ltime | Lmicroseconds | Llongfile, "XXX", "XXX" + Rdate + " " + Rtime + Rmicroseconds + " " + Rlongfile + " "},
	{Ldate | Ltime | Lmicroseconds | Lshortfile, "XXX", "XXX" + Rdate + " " + Rtime + Rmicroseconds + " " + Rshortfile + " "},
}

//
//// Test using Println("hello", 23, "world") or using Printf("hello %d world", 23)
//func testPrint(t *testing.T, flag int, prefix string, pattern string, useFormat bool) {
//	buf := new(bytes.Buffer)
//	SetOutput(buf)
//	SetFlags(flag)
//	SetPrefix(prefix)
//	if useFormat {
//		Printf("hello %d world", 23)
//	} else {
//		Println("hello", 23, "world")
//	}
//	line := buf.String()
//	line = line[0 : len(line)-1]
//	pattern = "^" + pattern + "hello 23 world$"
//	matched, err4 := regexp.MatchString(pattern, line)
//	if err4 != nil {
//		t.Fatal("pattern did not compile:", err4)
//	}
//	if !matched {
//		t.Errorf("log output should match %q is %q", pattern, line)
//	}
//	SetOutput(os.Stderr)
//}
//
//func TestAll(t *testing.T) {
//	for _, testcase := range tests {
//		testPrint(t, testcase.flag, testcase.prefix, testcase.pattern, false)
//		testPrint(t, testcase.flag, testcase.prefix, testcase.pattern, true)
//	}
//}

func TestOutput(t *testing.T) {
	const testString = "test"
	var b bytes.Buffer
	l := New(&b, LstdLevel, 0)
	l.Info(testString)
	if expect := "[INFO] " + testString + "\n"; b.String() != expect {
		t.Errorf("log output should match %q is %q", expect, b.String())
	}
}

//
//func TestFlagAndPrefixSetting(t *testing.T) {
//	var b bytes.Buffer
//	l := New(&b, "Test:", LstdLevels, LstdFlags)
//	f := l.Flags()
//	if f != LstdFlags {
//		t.Errorf("Flags 1: expected %x got %x", LstdFlags, f)
//	}
//	l.SetFlags(f | LUTC)
//	f = l.Flags()
//	if f != LstdFlags|LUTC {
//		t.Errorf("Flags 2: expected %x got %x", LstdFlags|LUTC, f)
//	}
//	p := l.Prefix()
//	if p != "Test:" {
//		t.Errorf(`Prefix: expected "Test:" got %q`, p)
//	}
//	l.SetPrefix("Reality:")
//	p = l.Prefix()
//	if p != "Reality:" {
//		t.Errorf(`Prefix: expected "Reality:" got %q`, p)
//	}
//
//	l.Info("hello")
//	t.Logf("%s", b.String())
//	pattern := "^Reality:" + Rdate + " " + Rtime + Rmicroseconds + " [INFO] hello\n"
//	matched, err := regexp.Match(pattern, b.Bytes())
//	if err != nil {
//		t.Fatalf("pattern %q did not compile: %s", pattern, err)
//	}
//	if !matched {
//		t.Error("message did not match pattern")
//	}
//}

func TestUTCFlag(t *testing.T) {
	var b bytes.Buffer
	l := New(&b, LstdLevel, Ldate|Ltime)
	l.SetFlags(Ldate | Ltime | LUTC)
	// Verify a log message looks right in the right time zone. Quantize to the second only.
	now := time.Now().UTC()
	l.Info("hello")
	want := fmt.Sprintf("%d/%.2d/%.2d %.2d:%.2d:%.2d [INFO] hello\n",
		now.Year(), now.Month(), now.Day(), now.Hour(), now.Minute(), now.Second())
	got := b.String()
	if got == want {
		return
	}
	// It's possible we crossed a second boundary between getting now and logging,
	// so add a second and try again. This should very nearly always work.
	now = now.Add(time.Second)
	want = fmt.Sprintf("%d/%.2d/%.2d %.2d:%.2d:%.2d [INFO] hello\n",
		now.Year(), now.Month(), now.Day(), now.Hour(), now.Minute(), now.Second())
	if got == want {
		return
	}
	t.Errorf("got %q; want %q", got, want)
}

func TestEmptyPrintCreatesLine(t *testing.T) {
	var b bytes.Buffer
	l := New(&b, LstdLevel, LstdFlags)
	l.Infof("")
	l.Info("non-empty")
	output := b.String()
	if n := strings.Count(output, "\n"); n != 2 {
		t.Errorf("expected 2 lines, got %d", n)
	}
}

func BenchmarkLogInfo(b *testing.B) {
	const test = "testing...."
	buff := bytes.NewBuffer(nil)
	logger := New(buff, LstdLevel, LstdFlags)
	for i := 0; i < b.N; i++ {
		buff.Reset()
		logger.Info(test)
	}
}

func BenchmarkLongFile(b *testing.B) {
	buff := bytes.NewBuffer(nil)
	logger := New(buff, LstdLevel, LstdFlags|Llongfile)
	for i := 0; i < b.N; i++ {
		buff.Reset()
		logger.Info("Should be discarded.")
	}
}

func BenchmarkLogWriteFile(b *testing.B) {
	logger := New(ioutil.Discard, LstdLevel, Lmicroseconds)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("Should be discarded.")
	}
}

func BenchmarkPrintFileName_Single(bb *testing.B) {
	logger := New(ioutil.Discard, LstdLevel, Lmicroseconds|Llongfile)

	var a int = 1
	var b float64 = 2.0
	var c string = "three"
	var d bool = true
	var e time.Duration = 5 * time.Second
	bb.ResetTimer()
	bb.StartTimer()
	for i := 0; i < bb.N; i++ {
		logger.Debug("Test logging, int:", a, ", float:", b, ", string:", c, ", bool:", d, ", time.Duration:", e)
	}
}

func BenchmarkParallelLogWriteFile(b *testing.B) {
	logger := New(ioutil.Discard, LstdLevel, Lmicroseconds|Llongfile)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			logger.Info("Should be discarded.")
		}
	})
}
