package log

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFileWriterDelOldFile(t *testing.T) {
	logdir := filepath.Join(os.TempDir(), "log_test")
	os.MkdirAll(logdir, os.ModeDir)
	oldfile := filepath.Join(logdir, "test.2016-01-02.001.log")
	fd, err := os.OpenFile(oldfile, os.O_CREATE, os.ModePerm)
	if err != nil {
		t.Fatal(err)
	}
	fd.Close()

	time.Sleep(1 * time.Second)

	defer os.Remove(oldfile)

	logfile := filepath.Join(logdir, "test.log")
	defer os.Remove(logfile)

	fw, err := NewFileWriter(logfile, ReserveDays(0))
	if err != nil {
		t.Fatal(err)
	}
	defer fw.Close()

	deleteOldFiles(fw.filename, fw.maxdays)

	_, err = os.Lstat(oldfile)
	if err == nil {
		t.Fatal(oldfile, "is exist")
	}
}

func TestFileWriterRotate(t *testing.T) {
	logdir, err := ioutil.TempDir(os.TempDir(), "log_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(logdir)

	logfile := filepath.Join(logdir, "test.log")
	fw, err := NewFileWriter(logfile, RotateByDaily(true), ReserveDays(1))
	if err != nil {
		t.Fatal(err)
	}
	defer fw.Close()

	line := "Hello, log.FileWriter"
	fw.Write([]byte(line))
	err = fw.doRotate()
	if err != nil {
		t.Fatal(err)
	}

	p := ""
	filepath.Walk(logdir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if info.Name() != "test.log" {
			p = path
		}
		return nil
	})

	data, err := ioutil.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != line {
		t.Fatal("data is different, failed to doRotate", p)
	}
}
