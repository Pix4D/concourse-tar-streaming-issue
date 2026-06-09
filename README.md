# concourse-tar-streaming-issue

Concourse pipeline demonstrating Windows volume stream-in behaviour (in >=v8.1) when a (git)
resource is fetched on Linux and streamed to a Windows worker.

Linux to Linux and Linux to Darwin streaming allow both cases below. Note that `tar` is preinstalled on a fresh `macOS` installation so by default all Darwin workers have
`tar` installed and follow the same streaming behavior as Linux (nothing is blocked, symlinks are not checked).

## Jobs

| Job | Verdict |
|-----|---------|
| `exfil-absolute-symlink-windows` | **Too loose** — `\Windows\...` symlink outside the input dir; `hosts` read succeeds |
| `safe-intra-repo-symlink-windows` | **False positive** — `../shared/...` stays inside the input dir; stream-in blocked |

The build logs for each job can be found in build-logs directory (main branch).

## Demo branches

The pipeline fetches one git branch per job so each Windows task streams only
the symlink(s) for that scenario. Both branches are orphan branches (created by
`ci/create-demo-branches.sh`) that contain just the fixture directory for their
job — not the full repo on `main`.

| Branch | Purpose |
|--------|---------|
| `demo/exfil-windows` | Supplies `exfil-windows/` for `exfil-absolute-symlink-windows` — symlink to `\Windows\...` outside the input dir |
| `demo/safe-intra-repo-windows` | Supplies `safe-intra-repo-windows/` for `safe-intra-repo-symlink-windows` — `../shared/...` symlink that stays inside the repo |

When stream-in fails on Windows, the task log only reports a generic
`failed to stream in to volume` — not which entry or symlink target was rejected.
The real reason (e.g. `entry './workers/images-vars.tf' links outside of target directory`)
is only in worker logs, so the build output alone cannot distinguish a false positive
from a correct block.

## Setup

```bash
chmod +x ci/create-demo-branches.sh
./ci/create-demo-branches.sh
git push -u origin main demo/exfil-windows demo/safe-intra-repo-windows

fly -t developers set-pipeline \
  -p concourse-tar-streaming-issue \
  -c ci/pipeline.yml \
  -l ci/vars.yml

fly -t developers unpause-pipeline -p concourse-tar-streaming-issue
```
