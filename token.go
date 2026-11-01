package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"golang.org/x/term"
)

var tokenRe = regexp.MustCompile(`^\d+:[A-Za-z0-9_-]{30,}$`)

// readToken берёт токен из systemd credential, если бот запущен как сервис;
// иначе спрашивает его в терминале без эха, а если stdin не терминал
// (pipe, фоновый процесс) — читает первую строку из stdin.
func readToken() (string, error) {
	var raw string
	fd := int(os.Stdin.Fd())
	if dir := os.Getenv("CREDENTIALS_DIRECTORY"); dir != "" {
		b, err := os.ReadFile(filepath.Join(dir, "token"))
		if err != nil {
			return "", fmt.Errorf("не удалось прочитать systemd credential: %w", err)
		}
		raw = string(b)
	} else if term.IsTerminal(fd) {
		fmt.Fprint(os.Stderr, "Токен бота (ввод скрыт): ")
		b, err := term.ReadPassword(fd)
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return "", fmt.Errorf("не удалось прочитать токен: %w", err)
		}
		raw = string(b)
	} else {
		line, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return "", fmt.Errorf("не удалось прочитать токен: %w", err)
		}
		raw = line
	}

	token := strings.TrimSpace(raw)
	if !tokenRe.MatchString(token) {
		return "", errors.New("это не похоже на токен бота (формат 123456789:AAE...)")
	}
	return token, nil
}

// redact вырезает токен из текста, который может попасть в лог.
func redact(s, token string) string {
	return strings.ReplaceAll(s, token, "***")
}
