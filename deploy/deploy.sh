#!/bin/bash
# Плавный деплой на сервере: новая версия поднимается рядом со старой, старая уходит только после
# того, как новая ответила на healthcheck. Хостовый nginx держит оба порта фронтенда (8090/8091),
# nginx фронтенда находит бэкенд по DNS Docker — поэтому 502 в момент переключения нет.
#   /opt/racion/deploy/deploy.sh            — бэкенд и фронтенд
#   /opt/racion/deploy/deploy.sh backend    — только бэкенд (деплой репозитория data)
set -euo pipefail
cd "${RACION_DIR:-/opt/racion}"
SERVICES=${*:-backend frontend}
HEALTH_URL=${HEALTH_URL:-https://racion.app/healthz}

docker compose build $SERVICES

roll() {
  local svc=$1 old new c
  old=$(docker compose ps -q "$svc" | tr '\n' ' ')
  docker compose up -d --no-deps --no-recreate --scale "$svc=2" "$svc"
  new=$(comm -13 <(echo "$old" | tr ' ' '\n' | sort) <(docker compose ps -q "$svc" | sort) | tr '\n' ' ')
  for c in $new; do
    for i in $(seq 1 60); do
      [ "$(docker inspect -f '{{.State.Health.Status}}' "$c")" = healthy ] && break
      sleep 2
      [ "$i" = 60 ] && { echo "$svc: new container not healthy"; docker logs --tail 30 "$c"; exit 1; }
    done
  done
  for c in $old; do docker stop -t 15 "$c" >/dev/null; docker rm "$c" >/dev/null; done
  docker compose up -d --no-deps --no-recreate --scale "$svc=1" "$svc" >/dev/null
  echo "$svc: rolled"
}

for svc in $SERVICES; do roll "$svc"; done
docker compose up -d --remove-orphans >/dev/null
docker image prune -f >/dev/null
sleep 2
curl -fsS -o /dev/null "$HEALTH_URL" && echo "healthz ok"
