# concourse-tar-streaming-issue

Concourse pipeline demonstrating Windows volume stream-in behaviour when a (git)
resource is fetched on Linux and streamed to a Windows worker.

The root cause is [Go issue #80073](https://github.com/golang/go/issues/80073):
`os.Root.Symlink` does not normalize path separators on Windows. During tar
extract on Windows (used by baggageclaim stream-in), Unix-style forward slashes
from the tar header are written verbatim into NTFS reparse points. Windows
requires backslashes for relative symlink targets, so the symlinks are
unreadable even though they stay inside the repo.

Linux-to-Linux and Linux-to-Darwin streaming are unaffected (`tar` is used and
symlink targets resolve correctly).

## Standalone reproducers

Two Go programs in this repo isolate the bug outside Concourse:

```bash
# Minimal comparison of os.Symlink vs os.Root.Symlink
GOOS=windows GOARCH=amd64 go build -o symlink-demo.exe symlink.go

# Full demo including robocopy (matches baggageclaim /sl flags)
GOOS=windows GOARCH=amd64 go build -o symlink-robocopy-demo.exe symlink-robocopy.go
```

Copy `symlink-robocopy-demo.exe` to a Windows worker and run it:

- **Windows Server 2019**: robocopy fails (exit code >= 2, ERROR 123 on broken
  directory symlinks)
- **Windows Server 2025**: robocopy succeeds (exit code <= 1) but symlinks
  created via `root.Symlink` with Unix targets remain unreadable

## Pipeline jobs

| Job | Worker | Verdict |
|-----|--------|---------|
| `exfil-absolute-symlink-windows` | 2025 | **Too loose** — `\Windows\...` symlink outside the input dir; `hosts` read succeeds |
| `safe-intra-repo-symlink-windows` | 2025 | **Go #80073 (file)** — stream-in succeeds; task fails reading the file symlink |
| `safe-intra-repo-dir-symlink-windows-2019` | 2019 | **Go #80073 (dir)** — stream-in fails; old robocopy cannot handle broken dir symlinks |
| `safe-intra-repo-dir-symlink-windows-2025` | 2025 | **Go #80073 (dir)** — stream-in succeeds; task fails listing the dir symlink |

Build logs for each job are in [`build-logs/`](build-logs/) (sanitized: no ANSI
codes, worker names, or internal hostnames).

When stream-in fails on Windows, the task log only reports a generic volume
error — typically `failed to stream in to volume` or `failed to create volume`
— not which entry or symlink target was rejected. The real reason is only in
worker logs (robocopy output for dir symlinks).

## Demo branches

The pipeline fetches one git branch per scenario so each Windows task streams
only the symlink(s) for that case. Branches are orphan branches (created by
`ci/create-demo-branches.sh`) containing just the fixture directory for their
scenario — not the full repo on `main`.

| Branch | Job(s) | Purpose |
|--------|--------|---------|
| `demo/exfil-windows` | `exfil-absolute-symlink-windows` | `\Windows\...` symlink outside the input dir |
| `demo/safe-intra-repo-windows` | `safe-intra-repo-symlink-windows` | File symlink `workers/images-vars.tf → ../shared/images-vars.tf` |
| `demo/safe-intra-repo-dir-windows` | `safe-intra-repo-dir-symlink-windows-2019`, `safe-intra-repo-dir-symlink-windows-2025` | Dir symlink `config/access/data-vault → ../../assets/bundle/user/embedded/data-vault` |

The dir fixture uses a deep `../../` target with the target tree under a prefix that walks before the link during tar creation, so Windows materializes a **directory** symlink (`d----l`), not a file symlink (`a----l`).

## Setup

```bash
chmod +x ci/create-demo-branches.sh
./ci/create-demo-branches.sh
git push -u origin main demo/exfil-windows demo/safe-intra-repo-windows demo/safe-intra-repo-dir-windows

TARGET=developers   # your Concourse fly target
fly -t "$TARGET" set-pipeline \
  -p concourse-tar-streaming-issue \
  -c ci/pipeline.yml \
  -l ci/vars.yml

fly -t "$TARGET" unpause-pipeline -p concourse-tar-streaming-issue
```
