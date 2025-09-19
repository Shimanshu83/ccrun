package main

import (
	"os"
)

const (
	CONTAINER_DIR  = "/opt/ccrun"
	ROOTFS_DIR     = "/opt/ccrun/rootfs"
	CONTAINER_NAME = "ccrun"
)

func prepareFile() error {

	err := makeDir(ROOTFS_DIR)
	if err != nil {
		return err
	}
	return nil

	// I have to download this base image for now and once this base image is downloaded
}

func makeDir(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return os.MkdirAll(path, 0755)
	} else {
		return err
	}
}
