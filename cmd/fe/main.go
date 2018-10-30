// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/pourer/pikamgr/config"
	"github.com/pourer/pikamgr/coordinate"
	"github.com/pourer/pikamgr/topom/dao"
	"github.com/pourer/pikamgr/utils/log"

	"github.com/go-martini/martini"
	"github.com/martini-contrib/render"
)

var roundTripper http.RoundTripper

func init() {
	var dials int64
	tr := &http.Transport{}
	tr.Dial = func(network, addr string) (net.Conn, error) {
		c, err := net.DialTimeout(network, addr, time.Second*10)
		if err == nil {
			log.Debugf("main: dial new connection to [%d] %s - %s", atomic.AddInt64(&dials, 1)-1, network, addr)
		}
		return c, err
	}
	go func() {
		for {
			time.Sleep(time.Minute)
			tr.CloseIdleConnections()
		}
	}()
	roundTripper = tr
}

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

	indexFile := filepath.Join(config.AssetsDir, "index.html")
	if _, err := os.Stat(indexFile); err != nil {
		log.Fatalf("main: get stat of %s failed", indexFile)
	}

	var loader ConfigLoader
	switch config.CoordinatorName {
	case "filesystem":
		loader = &StaticLoader{config.CoordinatorAddr}
		log.Infoln("main: set dashboard-list-file:", config.CoordinatorAddr)
	case "zookeeper", "etcd":
		log.Infof("main: set %s = %s", config.CoordinatorName, config.CoordinatorAddr)

		coordinator, err := coordinate.NewCoordinator(config.CoordinatorName, config.CoordinatorAddr, config.CoordinatorAuth, time.Minute)
		if err != nil {
			log.Errorf("main: create coordinator fail. coordinatorName-[%s] coordinatorAddr-[%s]", config.CoordinatorName, config.CoordinatorAddr)
			return
		}
		defer coordinator.Close()

		loader = &DynamicLoader{coordinator}
	default:
		log.Fatalln("main: unsupported coordinator. Only: filesystem zookeeper etcd")
	}

	router := NewReverseProxy(loader)

	m := martini.New()
	m.Use(martini.Recovery())
	m.Use(render.Renderer())
	m.Use(martini.Static(config.AssetsDir, martini.StaticOptions{SkipLogging: true}))

	r := martini.NewRouter()
	r.Get("/list", func() (int, string) {
		names := router.GetNames()
		sort.Sort(sort.StringSlice(names))

		data, err := json.MarshalIndent(names, "", "    ")
		if err != nil {
			return http.StatusInternalServerError, err.Error()
		}
		return http.StatusOK, string(data)
	})

	r.Any("/**", func(w http.ResponseWriter, req *http.Request) {
		name := req.URL.Query().Get("forward")
		if p := router.GetProxy(name); p != nil {
			p.ServeHTTP(w, req)
		} else {
			w.WriteHeader(http.StatusForbidden)
		}
	})

	m.MapTo(r, (*martini.Routes)(nil))
	m.Action(r.Handle)

	h := http.NewServeMux()
	h.Handle("/", m)

	server := &http.Server{
		Addr:    config.ListenAddr,
		Handler: h,
	}
	errChan := make(chan error, 1)
	go func() {
		errChan <- server.ListenAndServe()
	}()

	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, os.Kill, syscall.SIGTERM)
		defer signal.Stop(sigChan)

		errChan <- fmt.Errorf("%s", <-sigChan)
	}()

	log.Infoln("main: prepare exit:", <-errChan)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	server.Shutdown(ctx)

	log.Infoln("main: exit.")
}

func loadConfig(configFile string) (*config.FEConfig, error) {
	config := config.NewFEDefaultConfig()
	if err := config.LoadFromFile(configFile); err != nil {
		return nil, err
	}
	return config, nil
}

func initLog(config *config.FEConfig) (*log.FileWriter, error) {
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

type ConfigLoader interface {
	Reload() (map[string]string, error)
}

type StaticLoader struct {
	path string
}

func (l *StaticLoader) Reload() (map[string]string, error) {
	b, err := ioutil.ReadFile(l.path)
	if err != nil {
		return nil, err
	}
	var list []*struct {
		Name      string `json:"name"`
		Dashboard string `json:"dashboard"`
	}
	if err := json.Unmarshal(b, &list); err != nil {
		return nil, err
	}
	var m = make(map[string]string)
	for _, e := range list {
		m[e.Name] = e.Dashboard
	}
	return m, nil
}

type DynamicLoader struct {
	client coordinate.Client
}

func (l *DynamicLoader) Reload() (map[string]string, error) {
	var m = make(map[string]string)
	list, err := l.client.List(coordinate.ProductDir(), false)
	if err != nil {
		return nil, err
	}
	for _, path := range list {
		product := filepath.Base(path)
		if b, err := l.client.Read(coordinate.TopomPath(product), false); err != nil {
			log.Errorf("DynamicLoader::Reload read topom of product %s failed. err:%s", product, err.Error())
		} else if b != nil {
			var t = &dao.Topom{}
			if err := json.Unmarshal(b, t); err != nil {
				log.Errorln("DynamicLoader::Reload decode json failed. err:", err)
			} else {
				m[product] = t.AdminAddr
			}
		}
	}
	return m, nil
}

type ReverseProxy struct {
	sync.Mutex
	loadAt time.Time
	loader ConfigLoader
	routes map[string]*httputil.ReverseProxy
}

func NewReverseProxy(loader ConfigLoader) *ReverseProxy {
	r := &ReverseProxy{}
	r.loader = loader
	r.routes = make(map[string]*httputil.ReverseProxy)
	return r
}

func (r *ReverseProxy) reload(d time.Duration) {
	if time.Now().Sub(r.loadAt) < d {
		return
	}
	r.routes = make(map[string]*httputil.ReverseProxy)
	if m, err := r.loader.Reload(); err != nil {
		log.Errorln("ReverseProxy::reload reload reverse proxy failed. err:", err)
	} else {
		for name, host := range m {
			if name == "" || host == "" {
				continue
			}
			u := &url.URL{Scheme: "http", Host: host}
			p := httputil.NewSingleHostReverseProxy(u)
			p.Transport = roundTripper
			r.routes[name] = p
		}
	}
	r.loadAt = time.Now()
}

func (r *ReverseProxy) GetProxy(name string) *httputil.ReverseProxy {
	r.Lock()
	defer r.Unlock()
	return r.routes[name]
}

func (r *ReverseProxy) GetNames() []string {
	r.Lock()
	defer r.Unlock()
	r.reload(time.Second * 5)
	var names []string
	for name, _ := range r.routes {
		names = append(names, name)
	}
	return names
}
