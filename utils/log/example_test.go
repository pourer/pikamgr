package log_test

import (
	"io"
	"os"

	"github.com/pourer/pikamgr/utils/log"
)

func ExampleLogger() {
	// test2016-2-24.1.log
	fw, err := log.NewFileWriter("/tmp/log/test.log", log.RotateByDaily(true), log.ReserveDays(7), log.LogFileMaxSize(100))
	if err != nil {
		return
	}
	writer := io.MultiWriter(fw, os.Stdout)
	logger := log.New(writer, log.LstdLevel, log.LstdFlags)
	logger.Info("Hello, logger")
	logger.Debug("Hello, logger")
	logger.Error("Hello, logger")
	logger.Warn("Hello, logger")
	log.SetOutput(writer)
	log.SetLevel(log.LstdLevel)
	log.SetFlags(log.LstdFlags)
	log.Debug("Hello, standard logger")
	log.Info("Hello, standard logger")
}
