#!/usr/bin/env bash
# Create orphan demo branches so each job streams only its symlink(s).
# Run once after the initial commit on main:
#   ./ci/create-demo-branches.sh
set -euo pipefail

root=$(git rev-parse --show-toplevel)
cd "$root"

if ! git rev-parse --verify main &>/dev/null; then
  echo "error: commit to main first (git add . && git commit)" >&2
  exit 1
fi

make_branch() {
  local branch=$1
  shift
  local -a keep=("$@")

  echo "==> branch ${branch}"
  git checkout --orphan "$branch"
  git rm -rf --cached . >/dev/null 2>&1 || true
  rm -rf ./*
  rm -rf ./.gitignore 2>/dev/null || true
  git checkout main -- "${keep[@]}"
  git add .
  git commit -m "demo: ${branch} only"
}

make_branch demo/exfil-windows \
  exfil-windows

make_branch demo/safe-intra-repo-windows \
  safe-intra-repo-windows

git checkout main

branches=(
  demo/exfil-windows
  demo/safe-intra-repo-windows
)

echo
echo "Created branches:"
git branch --list 'demo/*'
echo
echo "Push:"
echo "  git push -u origin main ${branches[*]}"
