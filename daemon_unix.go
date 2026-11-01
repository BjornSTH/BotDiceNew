//go:build unix

package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

// detach перезапускает программу фоновым процессом в новой сессии.
// Токен передаётся через stdin-pipe — его не видно ни в аргументах,
// ни в окружении процесса (/proc/<pid>/cmdline, /proc/<pid>/environ).
// Ждёт, пока фоновый процесс подтвердит запуск через pipe на fd 3.
func detach(token, logPath string) (pid int, username string, err error) {
	exe, err := os.Executable()
	if err != nil {
		return 0, "", err
	}
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return 0, "", err
	}
	defer logFile.Close()

	readyR, readyW, err := os.Pipe()
	if err != nil {
		return 0, "", err
	}
	defer readyR.Close()

	cmd := exec.Command(exe, os.Args[1:]...)
	cmd.Env = append(os.Environ(), daemonEnv+"=1")
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.ExtraFiles = []*os.File{readyW}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return 0, "", err
	}
	if err := cmd.Start(); err != nil {
		return 0, "", err
	}
	readyW.Close()

	io.WriteString(stdin, token+"\n")
	stdin.Close()

	status, _ := io.ReadAll(readyR)
	reply := strings.TrimSpace(string(status))
	if name, ok := strings.CutPrefix(reply, "ok "); ok {
		pid = cmd.Process.Pid
		cmd.Process.Release()
		return pid, name, nil
	}

	cmd.Wait()
	if reply == "" {
		return 0, "", fmt.Errorf("фоновый процесс завершился, подробности в %s", logPath)
	}
	return 0, "", errors.New(strings.TrimPrefix(reply, "error "))
}

// runDaemon — точка входа фонового процесса, запущенного через detach.
func runDaemon() {
	ready := os.NewFile(3, "ready")
	token, err := readToken()
	if err == nil {
		err = run(token, func(username string) {
			log.Printf("Бот @%s запущен, PID %d", username, os.Getpid())
			fmt.Fprintf(ready, "ok %s\n", username)
			ready.Close()
		})
	}
	if err != nil {
		fmt.Fprintf(ready, "error %v\n", err)
		log.Fatal(err)
	}
}
