#!/usr/bin/env bash
#
# Script to mirror a tag from Prysm into ZondAPIs protocol buffers
#
# Example:
#
# mirror-zondapis.sh
#
set -e

# Validate settings.
[ "$TRACE" ] && set -x

## Define variables.
GH_API="https://api.github.com"
GH_REPO="$GH_API/repos/prysmaticlabs/zondapis"

AUTH="Authorization: token $GITHUB_SECRET_ACCESS_TOKEN"
## skipcq: SH-2034
export WGET_ARGS="--content-disposition --auth-no-challenge --no-cookie"
## skipcq: SH-2034
export CURL_ARGS="-LJO#"

## Validate token.
curl -o /dev/null -sH "$AUTH" "$GH_REPO" || { echo "Error: Invalid repo, token or network issue!";  exit 1; }

# Clone zondapis and qrysm
git clone https://github.com/theQRL/qrysm /tmp/qrysm/
git clone https://github.com/theQRL/zondapis /tmp/zondapis/

# Checkout the release tag in prysm and copy over protos
cd /tmp/prysm && git checkout "$BUILDKITE_COMMIT"

# Copy proto files, go files, and markdown files
find proto/zond \( -name '*.go' -o -name '*.proto' -o -name '*.md' \) -print0 |
    while IFS= read -r -d '' line; do
        item_path=$(dirname "$line")
        mkdir -p /tmp/zondapis"${item_path#*proto}" && cp "$line" /tmp/zondapis"${line#*proto}"
    done

cd /tmp/zondapis || exit

## Replace imports in go files and proto files as needed
find ./zond -name '*.go' -print0 |
    while IFS= read -r -d '' line; do
        sed -i 's/prysm\/proto\/zond/zondapis\/zond/g' "$line"
    done

find ./zond -name '*.go' -print0 |
    while IFS= read -r -d '' line; do
        sed -i 's/proto\/zond/zond/g' "$line"
    done

find ./zond -name '*.go' -print0 |
    while IFS= read -r -d '' line; do
        sed -i 's/proto_zond/zond/g' "$line"
    done

find ./zond -name '*.proto' -print0 |
    while IFS= read -r -d '' line; do
        sed -i 's/"proto\/zond/"zond/g' "$line"
    done

find ./zond -name '*.proto' -print0 |
    while IFS= read -r -d '' line; do
        sed -i 's/prysmaticlabs\/prysm\/proto\/zond/prysmaticlabs\/zondapis\/zond/g' "$line"
    done

if git diff-index --quiet HEAD --; then
   echo "nothing to push, exiting early"
   exit 0
else
   echo "changes detected, committing and pushing to zondapis"
fi

# Push to the mirror repository
git add --all
GIT_AUTHOR_EMAIL=contact@prysmaticlabs.com GIT_AUTHOR_NAME=prysm-bot GIT_COMMITTER_NAME=prysm-bot GIT_COMMITTER_EMAIL=contact@prysmaticlabs.com git commit -am "Mirrored from github.com/prysmaticlabs/prysm@$BUILDKITE_COMMIT"
git remote set-url origin https://prylabs:"$GITHUB_SECRET_ACCESS_TOKEN"@github.com/theQRL/zondapis.git
git push origin master
