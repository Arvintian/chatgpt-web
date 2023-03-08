package utils

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"k8s.io/klog/v2"
)

func ReplaceInFile(filePath string, targetStr string, replaceStr string) error {
	// 读取文件内容
	content, err := ioutil.ReadFile(filePath)
	if err != nil {
		return err
	}
	// 判断是否需要替换
	if !strings.Contains(string(content), targetStr) {
		return nil
	}

	// 替换字符串并写回文件，保持原有 filemode
	info, err := os.Stat(filePath)
	if err != nil {
		return err
	}
	newContent := strings.ReplaceAll(string(content), targetStr, replaceStr)
	err = ioutil.WriteFile(filePath, []byte(newContent), info.Mode())
	if err != nil {
		return err
	}
	klog.Infof("Replaced in file: %s\n", filePath)
	return nil
}

func ReplaceFiles(rootDir string, replacePairs map[string]string) error {
	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// 如果当前路径是目录，则继续遍历
		if info.IsDir() {
			return nil
		}
		// 处理文件
		for targetStr, replaceStr := range replacePairs {
			err = ReplaceInFile(path, targetStr, replaceStr)
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

func Reverse[S ~[]E, E any](s S) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}
