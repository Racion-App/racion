#!/bin/bash
# Установить от root в /usr/local/sbin/racion-release. Вызывается только как
# forced command отдельного ключа GitHub Actions: restrict,command="/usr/local/sbin/racion-release".
set -euo pipefail

if [[ ! ${SSH_ORIGINAL_COMMAND:-} =~ ^release\ ([0-9a-f]{40})$ ]]; then
  echo "Expected: release <main commit SHA>" >&2
  exit 2
fi
revision=${BASH_REMATCH[1]}

exec 8>/run/lock/racion-release.lock
flock -w 900 8
checkout=$(mktemp -d)
trap 'rm -rf -- "$checkout"' EXIT
git clone --quiet --depth 1 --single-branch --branch main \
  https://github.com/Racion-App/racion.git "$checkout"
if [[ $(git -C "$checkout" rev-parse HEAD) != "$revision" ]]; then
  echo "Requested revision is not the current main commit" >&2
  exit 1
fi

cd /opt/racion
test -s /etc/nginx/snippets/racion-active-frontend.conf
test -f docker-compose.override.yml
# Синхронизировать файлы между выкладками. deploy.sh берёт тот же lock на сборку и ротацию.
(
  flock -w 900 9
  rsync -a --delete \
    --exclude=.git --exclude=.gitmodules --exclude=.github \
    --exclude=/.env --exclude=/docker-compose.override.yml --exclude=/backups --exclude=/backup.sh \
    --exclude=node_modules --exclude=dist --exclude=.impeccable \
    --exclude=/backend/internal/seed/data --exclude=/frontend/public/images \
    "$checkout/" ./
) 9>/tmp/racion-deploy.lock
bash deploy/deploy.sh
curl -fsS --max-time 15 -o /dev/null https://racion.app/healthz
echo "Deployed $revision"
