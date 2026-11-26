#!/bin/bash
# Forced command for the data repository's CI key. The workflow supplies its short-lived, read-only GitHub token on stdin.
set -euo pipefail
[[ ${SSH_ORIGINAL_COMMAND:-} =~ ^release\ ([0-9a-f]{40})$ ]] || exit 2
revision=${BASH_REMATCH[1]}
exec 8>/run/lock/racion-release.lock
flock -w 900 8
checkout=$(mktemp -d)
trap 'rm -rf -- "$checkout"' EXIT
IFS= read -r DATA_TOKEN
[[ -n $DATA_TOKEN ]] || exit 2
export DATA_TOKEN
export GIT_TERMINAL_PROMPT=0
export GIT_ASKPASS="$checkout/askpass"
printf '%s\n' '#!/bin/sh' 'printf "%s\n" "$DATA_TOKEN"' > "$GIT_ASKPASS"
chmod 700 "$GIT_ASKPASS"
git -c credential.helper= -c credential.username=x-access-token clone --quiet --depth 1 --single-branch --branch main \
  https://github.com/Racion-App/data.git "$checkout/data"
unset DATA_TOKEN GIT_ASKPASS
[[ $(git -C "$checkout/data" rev-parse HEAD) == "$revision" ]] || { echo 'Requested revision is no longer main'; exit 1; }
cd /opt/racion
(
  flock -w 900 9
  # Only versioned seed JSON belongs in the Go build. Keep other files outside this sync.
  rsync -ac --delete --include='/*.json' --exclude='*' "$checkout/data/" backend/internal/seed/data/
) 9>/tmp/racion-deploy.lock
bash deploy/deploy.sh backend
echo "Deployed data $revision"
