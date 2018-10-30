package mapper

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/pourer/pikamgr/coordinate"
	"github.com/pourer/pikamgr/topom/dao"
	"github.com/pourer/pikamgr/utils/log"
)

type fileInfo struct {
	fullPath string
	os.FileInfo
}

type templateFileMapper struct {
	client  Client
	scanDir string
	mutex   *sync.Mutex
	tfs     dao.TemplateFiles
	done    chan struct{}
}

func NewTemplateFileMapper(client Client, scanDir string, interval time.Duration) (*templateFileMapper, error) {
	t := &templateFileMapper{
		client:  client,
		scanDir: scanDir,
		mutex:   new(sync.Mutex),
		tfs:     make(dao.TemplateFiles),
		done:    make(chan struct{}),
	}
	if err := t.init(); err != nil {
		return nil, err
	}

	go t.monitor(interval)
	return t, nil
}

func (m *templateFileMapper) init() error {
	tfDir := filepath.Dir(m.scanDir)
	if _, err := os.Stat(tfDir); err != nil {
		if err := os.MkdirAll(tfDir, os.ModePerm); err != nil {
			return err
		}
	}

	paths, err := m.client.List(coordinate.TemplateFileDir(), false)
	if err != nil {
		return err
	}

	tfs := make(dao.TemplateFiles)
	md5Calcer := md5.New()
	for _, path := range paths {
		data, err := m.client.Read(path, true)
		if err != nil {
			return err
		}

		md5Calcer.Reset()
		md5Value := md5Calcer.Sum(data)
		tfs[filepath.Base(path)] = &dao.TemplateFile{
			Data: data,
			MD5:  md5Value,
		}
	}

	files, err := m.getMatchFiles()
	if err != nil {
		return err
	}

	for fileBaseName, v := range tfs {
		if _, ok := files[fileBaseName]; ok {
			continue
		}

		if err := func(file string) error {
			f, err := os.OpenFile(file, os.O_RDWR|os.O_CREATE|os.O_TRUNC, os.ModePerm)
			if err != nil {
				return fmt.Errorf("open file-[%s] err-[%s]", file, err.Error())
			}
			defer f.Close()

			if _, err := f.Write(v.Data); err != nil {
				return fmt.Errorf("write file-[%s] err-[%s]", file, err.Error())
			}
			return nil
		}(filepath.Join(tfDir, fileBaseName)); err != nil {
			return err
		}
	}

	m.mutex.Lock()
	m.tfs = tfs
	m.mutex.Unlock()

	return nil
}

func (m *templateFileMapper) Close() error {
	close(m.done)
	return nil
}

func (m *templateFileMapper) monitor(interval time.Duration) {
	for {
		select {
		case <-m.done:
			return
		default:
		}

		if err := m.doMonitor(); err != nil {
			log.Errorln("templateFileMapper::monitor doMonitor fail. err:", err)
		}

		select {
		case <-m.done:
			return
		case <-time.After(interval):
		}
	}
}

func (m *templateFileMapper) doMonitor() error {
	files, err := m.getMatchFiles()
	if err != nil {
		return err
	}

	tfs := make(dao.TemplateFiles)
	md5Calcer := md5.New()
	for fileBaseName, info := range files {
		if err := func(file string) error {
			f, err := os.Open(file)
			if err != nil {
				return fmt.Errorf("open file-[%s] err-[%s]", file, err.Error())
			}
			defer f.Close()

			data, err := ioutil.ReadAll(f)
			if err != nil {
				return fmt.Errorf("read file-[%s] err-[%s]", file, err.Error())
			}

			md5Calcer.Reset()
			md5Value := md5Calcer.Sum(data)
			tfs[fileBaseName] = &dao.TemplateFile{
				Data: data,
				MD5:  md5Value,
			}
			return nil
		}(info.fullPath); err != nil {
			return err
		}
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()
	for fileBaseName := range m.tfs {
		if _, ok := tfs[fileBaseName]; ok {
			continue
		}

		m.delete(fileBaseName)
	}
	for fileBaseName, v := range tfs {
		if vv, ok := m.tfs[fileBaseName]; ok && bytes.Equal(vv.MD5, v.MD5) {
			continue
		}

		if err := m.update(fileBaseName, v); err != nil {
			delete(tfs, fileBaseName)
			continue
		}
	}
	m.tfs = tfs

	return nil
}

func (m *templateFileMapper) getMatchFiles() (map[string]*fileInfo, error) {
	matchFiles, err := filepath.Glob(m.scanDir)
	if err != nil {
		return nil, err
	}

	files := make(map[string]*fileInfo)
	for _, file := range matchFiles {
		info, err := os.Lstat(file)
		if err != nil {
			log.Errorln("templateFileMapper::doMonitor Lstat fail. err:", err, "file:", file)
			continue
		}

		if info.IsDir() {
			log.Infoln("templateFileMapper::doMonitor skipping directory:", file)
			continue
		}

		files[info.Name()] = &fileInfo{
			fullPath: file,
			FileInfo: info,
		}
	}

	return files, nil
}

func (m *templateFileMapper) update(fileName string, tf *dao.TemplateFile) error {
	log.Infof("templateFileMapper::update fileName-[%s]", fileName)

	if err := m.client.Update(coordinate.TemplateFilePath(fileName), tf.Data); err != nil {
		log.Errorln("templateFileMapper::update update fail. err:", err)
		return fmt.Errorf("templateFileMapper::update update fail. fileName-[%s] err-[%s]", fileName, err.Error())
	}
	return nil
}

func (m *templateFileMapper) delete(fileName string) error {
	log.Infof("templateFileMapper::delete fileName-[%s]\n", fileName)

	if err := m.client.Delete(coordinate.TemplateFilePath(fileName)); err != nil {
		log.Errorln("templateFileMapper::delete update fail. err:", err)
		return fmt.Errorf("templateFileMapper::delete update fail. fileName-[%s] err-[%s]", fileName, err.Error())
	}
	return nil
}

func (m *templateFileMapper) Info() (dao.TemplateFiles, error) {
	m.mutex.Lock()
	tfs := m.tfs
	m.mutex.Unlock()
	return tfs, nil
}
