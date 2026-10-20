#!/usr/bin/env bash
# Pull yetone/magpie's latest into this working copy, then rebase my work on
# top of it. Run from anywhere inside the repo.
#
#   ./sync-upstream.sh            sync main + rebase the branch I'm on
#   ./sync-upstream.sh --all      sync main + rebase every local my/* branch
#   ./sync-upstream.sh --check    只报告落后多少，不动任何分支
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"

BASE=main
UP=upstream/main

cur=$(git rev-parse --abbrev-ref HEAD)
dirty=$(git status --porcelain)

echo "→ fetching upstream…"
git fetch upstream --tags --prune

if [[ "${1:-}" == "--check" ]]; then
  behind=$(git rev-list --count "$BASE".."$UP")
  echo "本机 $BASE 落后 upstream $behind 个提交"
  for b in $(git for-each-ref --format='%(refname:short)' refs/heads/my/ 2>/dev/null); do
    echo "  $b 落后 $(git rev-list --count "$b".."$UP") 个提交"
  done
  exit 0
fi

if [[ -n "$dirty" ]]; then
  echo "✗ 工作区有未提交改动，先 commit 或 stash：" >&2
  echo "$dirty" >&2
  exit 1
fi

echo "→ fast-forwarding $BASE…"
git checkout -q "$BASE"
git merge --ff-only "$UP"
git push -q origin "$BASE"

if [[ "${1:-}" == "--all" ]]; then
  branches=$(git for-each-ref --format='%(refname:short)' refs/heads/my/ || true)
else
  branches=""
  [[ "$cur" == my/* ]] && branches="$cur"
fi

for b in $branches; do
  echo "→ rebasing $b onto $UP…"
  git checkout -q "$b"
  if git rebase "$UP"; then
    git push -q --force-with-lease origin "$b" && echo "  ✓ $b 已同步并推送"
  else
    echo "  ✗ $b 有冲突，解决后跑：git rebase --continue" >&2
    exit 1
  fi
done

git checkout -q "$cur"
echo "✓ 完成。当前停在 $cur"
