#!/bin/bash
# Плавный деплой на сервере: новая версия поднимается рядом со старой, старая уходит только после
# того, как новая ответила на healthcheck. Хостовый nginx переключается на порт готового фронтенда.
#   /opt/racion/deploy/deploy.sh            — бэкенд и фронтенд
#   /opt/racion/deploy/deploy.sh backend    — только бэкенд (деплой репозитория data)
set -euo pipefail
# Выкладки data и main, запущенные почти одновременно, мешали друг другу: одна снимала контейнер, который
# уже снимала другая («removal of container … is already in progress», 07.10). Вторая ждёт первую.
exec 9>/tmp/racion-deploy.lock
flock -w 900 9 || { echo "deploy: другая выкладка идёт дольше 15 минут"; exit 1; }
cd "${RACION_DIR:-/opt/racion}"
SERVICES=${*:-backend frontend}
HEALTH_URL=${HEALTH_URL:-https://racion.app/healthz}

docker compose build $SERVICES

active_conf=/etc/nginx/snippets/racion-active-frontend.conf

frontend_port() {
  local port
  port=$(docker inspect -f '{{(index (index .NetworkSettings.Ports "80/tcp") 0).HostPort}}' "$1") || return 1
  case "$port" in 8090|8091) echo "$port";; *) echo "unexpected frontend port: $port" >&2; return 1;; esac
}

switch_frontend() {
  local port=$1 workers pending previous backup i
  curl -fsS --max-time 5 -o /dev/null "http://127.0.0.1:$port/healthz" || return 1
  workers=$(pgrep -d, -P "$(cat /run/nginx.pid)" -f '^nginx: worker process') || return 1
  previous=$(cat "$active_conf") || return 1
  pending=$(mktemp "${active_conf}.XXXXXX") || return 1
  if [[ "$port" == 8090 ]]; then backup=8091; else backup=8090; fi
  printf 'server 127.0.0.1:%s;\nserver 127.0.0.1:%s backup;\n' "$port" "$backup" > "$pending"
  chmod 644 "$pending"
  mv -f "$pending" "$active_conf"
  if ! nginx -t || ! systemctl reload nginx; then
    printf '%s\n' "$previous" > "$active_conf"
    nginx -t && systemctl reload nginx
    return 1
  fi
  # При reload старые воркеры завершают начатые запросы через предыдущий контейнер.
  for ((i=0; i<120; i++)); do
    if ! ps -p "$workers" -o pid= >/dev/null; then return 0; fi
    sleep 1
  done
  echo "nginx: previous workers still draining; keeping both frontends" >&2
  return 1
}

roll() {
  local svc=$1 old new c i port keep
  old=$(docker compose ps -q "$svc" | tr '\n' ' ')
  if [[ "$svc" == frontend && -f "$active_conf" && $(wc -w <<< "$old") -gt 1 ]]; then
    # Повтор после прерывания: подтвердить рабочий порт до удаления лишней реплики.
    keep=""
    for c in $old; do
      port=$(frontend_port "$c")
      if grep -Fxq "server 127.0.0.1:$port;" "$active_conf"; then keep=$c; fi
    done
    [[ -n "$keep" ]] || { echo "cannot identify active frontend" >&2; return 1; }
    switch_frontend "$(frontend_port "$keep")"
    for c in $old; do
      if [[ "$c" != "$keep" ]]; then docker stop -t 15 "$c" >/dev/null; docker rm "$c" >/dev/null; fi
    done
    old=$keep
  fi
  if [[ -z "${old// /}" ]]; then
    docker compose up -d --no-deps --wait "$svc"
    if [[ "$svc" == frontend && -f "$active_conf" ]]; then
      switch_frontend "$(frontend_port "$(docker compose ps -q frontend)")"
    fi
    return
  fi
  docker compose up -d --no-deps --no-recreate --scale "$svc=2" "$svc"
  new=$(comm -13 <(echo "$old" | tr ' ' '\n' | sort) <(docker compose ps -q "$svc" | sort) | tr '\n' ' ')
  [[ $(wc -w <<< "$new") == 1 ]] || { echo "$svc: expected one new container; keeping current replicas" >&2; return 1; }
  for c in $new; do
    for i in $(seq 1 60); do
      [ "$(docker inspect -f '{{.State.Health.Status}}' "$c")" = healthy ] && break
      sleep 2
      [ "$i" = 60 ] && { echo "$svc: new container not healthy"; docker logs --tail 30 "$c"; exit 1; }
    done
  done
  if [[ "$svc" == frontend && -f "$active_conf" ]]; then
    switch_frontend "$(frontend_port "$c")"
  fi
  for c in $old; do docker stop -t 15 "$c" >/dev/null; docker rm "$c" >/dev/null; done
  docker compose up -d --no-deps --no-recreate --scale "$svc=1" "$svc" >/dev/null
  echo "$svc: rolled"
}

for svc in $SERVICES; do roll "$svc"; done
docker compose up -d --remove-orphans >/dev/null
docker image prune -f >/dev/null
sleep 2
curl -fsS -o /dev/null "$HEALTH_URL" && echo "healthz ok"
