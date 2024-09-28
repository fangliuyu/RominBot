// Package file 文件实用工具
package file

import "os"

// Pwd 获取当前路径
func Pwd() (path string) {
	path, _ = os.Getwd()
	return
}

// BOTPATH BOT当前路径
var BOTPATH = Pwd()

// DATAPATH data数据路径
var DATAPATH = Pwd() + "/data"

// IsExist 文件/路径存在
func IsExist(path string) bool {
	_, err := os.Stat(path)
	return err == nil || os.IsExist(err)
}

// IsNotExist 文件/路径不存在
func IsNotExist(path string) bool {
	_, err := os.Stat(path)
	return err != nil && os.IsNotExist(err)
}

// Size 获取文件大小
func Size(path string) (n int64) {
	stat, err := os.Stat(path)
	if err != nil {
		return
	}
	n = stat.Size()
	return
}
