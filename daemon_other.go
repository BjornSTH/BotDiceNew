//go:build !unix

package main

import "errors"

func detach(token, logPath string) (int, string, error) {
	return 0, "", errors.New("фоновый режим поддерживается только на Linux/macOS, запустите с флагом -fg")
}

func runDaemon() {}
