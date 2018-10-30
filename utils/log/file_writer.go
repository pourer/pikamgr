package log

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	DefaultRotateByDaily   = true
	DefaultRotateByMaxSize = 100 // MB
	DefaultReserveMaxDays  = 7
)

// FileWriter implements io.WriteCloser. You can use FileWriterOption config it.
// A FileWriter can be used simultaneously from multiple goroutines;
// it guarantees to serialize access to the Writer.
type FileWriter struct {
	filename      string
	mu            sync.Mutex
	fd            *os.File
	rotatebydaily bool
	maxdays       int
	openTime      time.Time
	maxsize       int64
	currentSize   int64
	done          chan struct{}
}

type FileWriterOption func(*FileWriter) error

// RotateByDaily configs FileWriter rotates logger files
func RotateByDaily(enable bool) FileWriterOption {
	return func(f *FileWriter) error {
		f.rotatebydaily = enable
		return nil
	}
}

// ReserveDays reserver logger file maxdays
func ReserveDays(maxdays int) FileWriterOption {
	return func(f *FileWriter) error {
		f.maxdays = maxdays
		return nil
	}
}

// LogFileMaxSize rotates logger file by size(MB), if size equals or less than 0, will disable the future.
func LogFileMaxSize(size int) FileWriterOption {
	return func(f *FileWriter) error {
		f.maxsize = int64(size) * 1024 * 1024
		return nil
	}
}

// NewFileWriter creates new FileWriter, filename must be full path
func NewFileWriter(filename string, options ...FileWriterOption) (*FileWriter, error) {
	f := &FileWriter{
		filename:      filename,
		rotatebydaily: DefaultRotateByDaily,
		maxdays:       DefaultReserveMaxDays,
		maxsize:       DefaultRotateByMaxSize * 1024 * 1024,
		done:          make(chan struct{}),
	}

	for _, opt := range options {
		if err := opt(f); err != nil {
			return nil, err
		}
	}

	err := os.MkdirAll(filepath.Dir(filename), os.ModePerm)
	if err != nil {
		return nil, err
	}

	fd, err := os.OpenFile(filename, os.O_CREATE|os.O_APPEND|os.O_WRONLY, os.ModePerm)
	if err != nil {
		return nil, err
	}

	f.fd = fd
	f.initStat()

	go f.deleteOldFiles()

	return f, nil
}

// Close close FileWriter.
func (f *FileWriter) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	close(f.done)
	return f.fd.Close()
}

func (f *FileWriter) needRotate() bool {
	now := time.Now()
	if (f.maxsize > 0 && f.currentSize >= f.maxsize) ||
		(f.rotatebydaily && now.Day() != f.openTime.Day()) {
		return true
	}
	return false
}

func (f *FileWriter) doRotate() error {
	if _, err := os.Lstat(f.filename); err != nil {
		return err
	}

	ext := filepath.Ext(f.filename)
	prefix := strings.TrimSuffix(f.filename, ext)
	if ext == "" {
		ext = ".log"
	}

	date := f.openTime.Format("20060102-150405.000000")
	newfilename := fmt.Sprintf("%s.%s%s", prefix, date, ext)

	f.fd.Close()
	os.Rename(f.filename, newfilename)
	fd, err := os.OpenFile(f.filename, os.O_APPEND|os.O_WRONLY|os.O_CREATE, os.ModePerm)
	if err != nil {
		return err
	}
	f.fd = fd
	f.initStat()

	return nil
}

func (f *FileWriter) initStat() {
	if stat, err := f.fd.Stat(); err == nil {
		f.currentSize = stat.Size()
	}
	f.openTime = time.Now()
}

func deleteOldFiles(filename string, maxdays int) {
	ext := filepath.Ext(filename)
	prefix := strings.TrimSuffix(filename, ext)

	filepath.Walk(filepath.Dir(filename), func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if path == filename {
			return nil
		}

		if !info.IsDir() && info.ModTime().Unix() < time.Now().Unix()-60*60*24*int64(maxdays) {
			if strings.HasPrefix(path, prefix) && strings.HasSuffix(path, ext) {
				os.Remove(path)
			}
		}
		return nil
	})
}

func (f *FileWriter) deleteOldFiles() {
	duration := 1 * time.Minute
	timer := time.NewTimer(duration)
	for {
		select {
		case <-timer.C:
			deleteOldFiles(f.filename, f.maxdays)
			timer.Reset(duration)
		case <-f.done:
			return
		}
	}
}

// Write writes len(b) bytes to the FileWriter.
func (f *FileWriter) Write(b []byte) (int, error) {
	f.mu.Lock()

	if f.needRotate() {
		if err := f.doRotate(); err != nil {
			fmt.Fprintf(os.Stderr, "log: failed to doRotate, err: %s", err.Error())
		}
	}

	n, err := f.fd.Write(b)
	if err == nil {
		f.currentSize += int64(n)
	}
	f.mu.Unlock()
	return n, err
}
