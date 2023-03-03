package main

import (
	"io/ioutil"
	"path/filepath"
	"strings"

	"k8s.io/klog/v2"
)

func editPathFiles(path string, old string, new string) error {
	files, err := ioutil.ReadDir(path)
	if err != nil {
		return err
	}
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		path := filepath.Join(path, file.Name())
		bts, err := ioutil.ReadFile(path)
		if err != nil {
			return err
		}
		content := string(bts)
		if !strings.Contains(content, old) {
			continue
		}
		content = strings.ReplaceAll(content, old, new)
		if err = ioutil.WriteFile(path, []byte(content), file.Mode()); err != nil {
			return err
		}
		klog.Infof("Edited %s", file.Name())
	}
	return nil
}
