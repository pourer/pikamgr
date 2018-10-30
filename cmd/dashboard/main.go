package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/pourer/pikamgr/config"
	"github.com/pourer/pikamgr/coordinate"
	"github.com/pourer/pikamgr/handler"
	"github.com/pourer/pikamgr/topom"
	"github.com/pourer/pikamgr/topom/dao/mapper"
	"github.com/pourer/pikamgr/utils/log"

	"github.com/gin-gonic/gin"
)

func main() {
	var configFile string
	flag.StringVar(&configFile, "c", "", "must specifie the config file")
	flag.Parse()
	if configFile == "" {
		flag.Usage()
		return
	}

	config, err := loadConfig(configFile)
	if err != nil {
		log.Fatal("main: loadConfig fail. err:", err)
	}
	logFile, err := initLog(config)
	if err != nil {
		log.Fatal("main: initLog fail. err:", err)
	}
	defer logFile.Close()

	coordinator, err := coordinate.NewCoordinator(config.CoordinatorName, config.CoordinatorAddr, config.CoordinatorAuth, time.Minute)
	if err != nil {
		log.Errorf("main: create coordinator fail. coordinatorName-[%s] coordinatorAddr-[%s]", config.CoordinatorName, config.CoordinatorAddr)
		return
	}
	defer coordinator.Close()

	topomMapper, err := mapper.NewTopomMapper(config.ProductName, config.AdminAddr, coordinator)
	if err != nil {
		log.Errorln("main: NewTopomMapper fail. err:", err)
		return
	}
	groupMapper, err := mapper.NewGroupMapper(config.ProductName, coordinator)
	if err != nil {
		log.Errorln("main: NewGroupMapper fail. err:", err)
		return
	}
	sentinelMapper, err := mapper.NewSentinelMapper(config.ProductName, coordinator)
	if err != nil {
		log.Errorln("main: NewSentinelMapper fail. err:", err)
		return
	}
	gslbMapper, err := mapper.NewGSLBMapper(config.ProductName, coordinator, groupMapper)
	if err != nil {
		log.Errorln("main: NewGSLBMapper fail. err:", err)
		return
	}
	templateFileMapper, err := mapper.NewTemplateFileMapper(coordinator, config.TemplateFileScanDir, config.TemplateFileScanInterval.Duration())
	if err != nil {
		log.Errorln("main: NewTemplateFileMapper fail. err:", err)
		return
	}
	defer templateFileMapper.Close()

	service, err := topom.NewService(config, topomMapper, groupMapper, sentinelMapper, gslbMapper, templateFileMapper)
	if err != nil {
		log.Errorln("main: NewService fail. err:", err)
		return
	}
	defer service.Close()

	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()
	r.Use(handler.RecordSourceHandler, handler.GzipHandler)

	apiRouter := r.Group("/api/topom")
	handler.InitAggHandler(service, r, apiRouter)
	handler.InitGroupHandler(service, apiRouter)
	handler.InitSentinelHandler(service, apiRouter)
	handler.InitGSLBHandler(service, apiRouter)
	handler.InitTFHandler(service, apiRouter)

	server := &http.Server{
		Addr:    config.AdminAddr,
		Handler: r,
	}

	errChan := make(chan error, 1)
	go func() {
		defer service.Close()
		errChan <- server.ListenAndServe()
	}()

	go func() {
		defer service.Close()

		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, os.Kill, syscall.SIGTERM)
		defer signal.Stop(sigChan)

		errChan <- fmt.Errorf("%s", <-sigChan)
	}()

	if err := service.Start(); err != nil {
		log.Errorln("main: service Start fail. err:", err)
	} else {
		log.Infoln("main: prepare exit:", <-errChan)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	server.Shutdown(ctx)

	for service.IsOnline() {
		time.Sleep(time.Second)
	}

	log.Infoln("main: exit.")
}

func loadConfig(configFile string) (*config.DashboardConfig, error) {
	config := config.NewDashboardDefaultConfig()
	if err := config.LoadFromFile(configFile); err != nil {
		return nil, err
	}
	return config, nil
}

func initLog(config *config.DashboardConfig) (*log.FileWriter, error) {
	logfile, err := log.NewFileWriter(config.LogFilePath,
		log.ReserveDays(config.LogReserveDays),
		log.LogFileMaxSize(config.LogMaxSize))
	if err != nil {
		return nil, err
	}

	var writer io.Writer = logfile
	if config.LogPrintScreen {
		writer = io.MultiWriter(logfile, os.Stdout)
	}
	log.SetOutput(writer)
	log.SetLevel(log.StringToLevel(config.LogLevel))
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	log.Info("main: initLog init log succeed")
	return logfile, nil
}
