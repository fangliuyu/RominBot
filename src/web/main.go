// Package web 网络处理相关
package web

import (
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net/http"
)

// NewDefaultClient ...
func NewDefaultClient() *http.Client {
	return &http.Client{}
}

// NewTLS12Client ...
func NewTLS12Client() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				MaxVersion: tls.VersionTLS12,
			},
		},
	}
}

// RequestDataWith 使用自定义请求头获取数据
func RequestDataWith(client *http.Client, url, method, referer, ua string, body io.Reader) (data []byte, err error) {
	// 提交请求
	var request *http.Request
	request, err = http.NewRequest(method, url, body)
	if err == nil {
		// 增加header选项
		if referer != "" {
			request.Header.Add("Referer", referer)
		}
		if ua != "" {
			request.Header.Add("User-Agent", ua)
		}
		var response *http.Response
		response, err = client.Do(request)
		if err == nil {
			if response.StatusCode != http.StatusOK {
				s := fmt.Sprintf("status code: %d", response.StatusCode)
				err = errors.New(s)
				return
			}
			data, err = io.ReadAll(response.Body)
			response.Body.Close()
		}
	}
	return
}

// RequestDataWithHeaders 使用自定义请求头获取数据
func RequestDataWithHeaders(client *http.Client, url, method string, setheaders func(*http.Request) error, body io.Reader) (data []byte, err error) {
	// 提交请求
	var request *http.Request
	request, err = http.NewRequest(method, url, body)
	if err == nil {
		// 增加header选项
		err = setheaders(request)
		if err != nil {
			return
		}
		var response *http.Response
		response, err = client.Do(request)
		if err != nil {
			return
		}
		if response.StatusCode != http.StatusOK {
			s := fmt.Sprintf("status code: %d", response.StatusCode)
			err = errors.New(s)
			return
		}
		data, err = io.ReadAll(response.Body)
		response.Body.Close()
	}
	return
}

// GetData 获取数据
func GetData(url string) (data []byte, err error) {
	var response *http.Response
	response, err = http.Get(url)
	if err == nil {
		if response.StatusCode != http.StatusOK {
			s := fmt.Sprintf("status code: %d", response.StatusCode)
			err = errors.New(s)
			return
		}
		data, err = io.ReadAll(response.Body)
		response.Body.Close()
	}
	return
}

// PostData 获取数据
func PostData(url, contentType string, body io.Reader) (data []byte, err error) {
	var response *http.Response
	response, err = http.Post(url, contentType, body)
	if err == nil {
		if response.StatusCode != http.StatusOK {
			s := fmt.Sprintf("status code: %d", response.StatusCode)
			err = errors.New(s)
			return
		}
		data, err = io.ReadAll(response.Body)
		response.Body.Close()
	}
	return
}

// HeadRequestURL 获取跳转后的链接
func HeadRequestURL(u string) (string, error) {
	data, err := http.Head(u)
	if err != nil {
		return "", err
	}
	_ = data.Body.Close()
	return data.Request.URL.String(), nil
}
