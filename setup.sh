#!/usr/bin/env bash
# Установка BotDice как systemd-сервиса с автозапуском.
#
#   ./setup.sh            установить или обновить (повторный запуск пересобирает бинарник)
#   ./setup.sh token      заменить токен
#   ./setup.sh uninstall  удалить сервис, бинарник и токен
#
# Нужен Go для сборки. Если на сервере его нет, соберите на своей машине
# (GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o botdice .)
# и положите бинарник botdice рядом со скриптом.
set -euo pipefail

SERVICE=botdice
BIN=/usr/local/bin/botdice
UNIT=/etc/systemd/system/$SERVICE.service
CRED_ENC=/etc/credstore.encrypted/$SERVICE.token
CRED_PLAIN=/etc/credstore/$SERVICE.token

SRC_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
ACTION=${1:-install}

say()  { printf '\033[1;32m==>\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33m!!\033[0m %s\n' "$*" >&2; }
die()  { printf '\033[1;31mОшибка:\033[0m %s\n' "$*" >&2; exit 1; }

case $ACTION in
	install|token|uninstall) ;;
	*) die "неизвестная команда «$ACTION». Используйте: install, token или uninstall" ;;
esac

# Сборку делаем от обычного пользователя (у root часто нет Go в PATH),
# затем перезапускаемся через sudo.
build() {
	if command -v go >/dev/null 2>&1; then
		say "Сборка бинарника"
		(cd "$SRC_DIR" && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o botdice .)
	elif [[ -x $SRC_DIR/botdice ]]; then
		warn "Go не найден, используется готовый бинарник $SRC_DIR/botdice"
	else
		die "нет ни Go, ни готового бинарника $SRC_DIR/botdice"
	fi
}

if [[ $EUID -ne 0 ]]; then
	[[ $ACTION == install ]] && build
	command -v sudo >/dev/null 2>&1 || die "запустите скрипт от root"
	exec sudo bash "${BASH_SOURCE[0]}" "$ACTION" --built
fi

command -v systemctl >/dev/null 2>&1 || die "systemd не найден"

uninstall() {
	say "Удаление сервиса $SERVICE"
	systemctl disable --now "$SERVICE" 2>/dev/null || true
	rm -f "$UNIT" "$BIN" "$CRED_ENC" "$CRED_PLAIN"
	systemctl daemon-reload
	say "Готово. Не забудьте отозвать токен в @BotFather (/revoke), если он больше не нужен."
}

# Читает токен со скрытым вводом и проверяет его через getMe.
ask_token() {
	local token
	while true; do
		read -rsp "Токен бота (ввод скрыт): " token </dev/tty
		echo
		token=${token//[[:space:]]/}
		if [[ ! $token =~ ^[0-9]+:[A-Za-z0-9_-]{30,}$ ]]; then
			warn "Это не похоже на токен (формат 123456789:AAE...). Попробуйте ещё раз."
			continue
		fi
		if command -v curl >/dev/null 2>&1; then
			# URL передаётся curl через stdin, а не аргументом, чтобы токен не светился в ps.
			local resp
			resp=$(printf 'url = "https://api.telegram.org/bot%s/getMe"\n' "$token" |
				curl -sS --max-time 15 -K - 2>&1) || true
			if [[ $resp == *'"ok":true'* ]]; then
				local name=${resp#*\"username\":\"}
				say "Токен принят: @${name%%\"*}"
			elif [[ $resp == *'"ok":false'* ]]; then
				warn "Telegram отклонил токен. Попробуйте ещё раз."
				continue
			else
				warn "Не удалось связаться с Telegram, токен не проверен: ${resp:-нет ответа}"
			fi
		fi
		TOKEN=$token
		return
	done
}

# Сохраняет токен как systemd credential: зашифрованным, если systemd это умеет,
# иначе файлом, доступным только root.
store_token() {
	rm -f "$CRED_ENC" "$CRED_PLAIN"
	if systemd-creds --help 2>/dev/null | grep -q '^ *encrypt'; then
		install -d -m 0700 "$(dirname "$CRED_ENC")"
		printf '%s' "$TOKEN" | systemd-creds encrypt --name=token - "$CRED_ENC" >/dev/null
		chmod 0600 "$CRED_ENC"
		say "Токен зашифрован и сохранён в $CRED_ENC"
	else
		install -d -m 0700 "$(dirname "$CRED_PLAIN")"
		(umask 077 && printf '%s' "$TOKEN" >"$CRED_PLAIN")
		warn "systemd-creds недоступен, токен сохранён в $CRED_PLAIN (доступ только root)"
	fi
	unset TOKEN
}

cred_line() {
	if [[ -f $CRED_ENC ]]; then
		echo "LoadCredentialEncrypted=token:$CRED_ENC"
	else
		echo "LoadCredential=token:$CRED_PLAIN"
	fi
}

write_unit() {
	cat >"$UNIT" <<EOF
[Unit]
Description=BotDice — Telegram-бот для бросков кубиков
Wants=network-online.target
After=network-online.target

[Service]
Type=simple
ExecStart=$BIN -fg
$(cred_line)
Restart=always
RestartSec=10

DynamicUser=yes
NoNewPrivileges=yes
ProtectSystem=strict
ProtectHome=yes
PrivateTmp=yes
PrivateDevices=yes
ProtectKernelTunables=yes
ProtectKernelModules=yes
ProtectKernelLogs=yes
ProtectControlGroups=yes
ProtectClock=yes
ProtectHostname=yes
RestrictAddressFamilies=AF_INET AF_INET6 AF_UNIX
RestrictNamespaces=yes
RestrictRealtime=yes
LockPersonality=yes
MemoryDenyWriteExecute=yes
SystemCallArchitectures=native
CapabilityBoundingSet=
MemoryMax=256M

[Install]
WantedBy=multi-user.target
EOF
}

start_service() {
	systemctl daemon-reload
	systemctl stop "$SERVICE" 2>/dev/null || true
	if pgrep -x botdice >/dev/null; then
		warn "Уже запущен другой процесс botdice (PID $(pgrep -x botdice | tr '\n' ' '))."
		warn "Два бота с одним токеном мешают друг другу — остановите его: kill <PID>"
	fi
	systemctl enable --now "$SERVICE" >/dev/null
	sleep 3
	if systemctl is-active --quiet "$SERVICE"; then
		say "Бот работает и будет запускаться автоматически"
	else
		warn "Сервис не запустился, последние строки журнала:"
		journalctl -u "$SERVICE" -n 20 --no-pager || true
		exit 1
	fi
}

case $ACTION in
uninstall)
	uninstall
	;;
token)
	[[ -f $UNIT ]] || die "бот ещё не установлен, запустите ./setup.sh"
	ask_token
	store_token
	write_unit
	start_service
	;;
install)
	[[ ${2:-} == --built ]] || build
	[[ -x $SRC_DIR/botdice ]] || die "бинарник $SRC_DIR/botdice не найден"

	say "Установка бинарника в $BIN"
	install -m 0755 "$SRC_DIR/botdice" "$BIN"

	if [[ -f $CRED_ENC || -f $CRED_PLAIN ]]; then
		read -rp "Токен уже сохранён. Заменить? [y/N] " answer </dev/tty
		if [[ $answer == [yYдД]* ]]; then
			ask_token
			store_token
		fi
	else
		ask_token
		store_token
	fi

	write_unit
	start_service
	cat <<EOF

Полезные команды:
  systemctl status $SERVICE       состояние
  journalctl -u $SERVICE -f       лог в реальном времени
  sudo systemctl restart $SERVICE перезапуск
  ./setup.sh token                заменить токен
  ./setup.sh uninstall            удалить
EOF
	;;
esac
