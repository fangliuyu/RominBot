// Package file 文件实用工具
package file

import (
	"crypto/tls"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/RomiChan/syncx"
)

type dlcache syncx.Map[string, error]

func (dlc *dlcache) wait(url string) error {
	if err, loaded := (*syncx.Map[string, error])(dlc).LoadOrStore(url, errDlStatusDoing); loaded {
		if err != errDlStatusDoing {
			return err
		}
		t := time.NewTicker(time.Second)
		defer t.Stop()
		i := 0
		for range t.C {
			if err, ok := (*syncx.Map[string, error])(dlc).Load(url); ok && err != errDlStatusDoing {
				return err
			}
			i++
			if i > 60 {
				break
			}
		}
		return errDlStatusTimeout
	}
	time.AfterFunc(time.Minute*2, func() {
		(*syncx.Map[string, error])(dlc).Delete(url)
	})
	return errDlContinue
}

func (dlc *dlcache) set(url string, err error) {
	(*syncx.Map[string, error])(dlc).Store(url, err)
}

var (
	tr = &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	nochkcrtcli = &http.Client{Transport: tr}
	dlmap       = dlcache{}
)

var (
	errDlContinue      = errors.New("continue")
	errDlStatusDoing   = errors.New("downloading")
	errDlStatusTimeout = errors.New("download timeout")
)

// DownloadTo 下载到路径
func DownloadTo(url, path string) (filePath string, err error) {
	err = dlmap.wait(url)
	if err != errDlContinue {
		return
	}
	resp, err := http.Get(url)
	if err == nil {
		var f *os.File
		info, err1 := os.Stat(path)
		if err1 != nil {
			err = err1
			return
		}
		if info.IsDir() {
			if IsNotExist(path) {
				err = os.Mkdir(path, 0755)
				if err != nil {
					return
				}
			}
			fileInfo := resp.Header["Content-Disposition"][0]
			fileName := strings.Replace(fileInfo[strings.LastIndex(fileInfo, "=")+1:], "\"", "", -1)
			filePath = filepath.Join(path, fileName)
		} else {
			filePath = path
		}
		f, err = os.Create(filePath)
		if err == nil {
			_, err = io.Copy(f, resp.Body)
			f.Close()
		}
		resp.Body.Close()
	}
	dlmap.set(url, err)
	return
}

// NoChkCrtDownloadTo 下载到路径
func NoChkCrtDownloadTo(url, file string) error {
	err := dlmap.wait(url)
	if err != errDlContinue {
		return err
	}
	resp, err := nochkcrtcli.Get(url)
	if err == nil {
		var f *os.File
		f, err = os.Create(file)
		if err == nil {
			_, err = io.Copy(f, resp.Body)
			f.Close()
		}
		resp.Body.Close()
	}
	dlmap.set(url, err)
	return err
}
