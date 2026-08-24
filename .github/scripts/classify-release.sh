#!/usr/bin/env bash
# Classify a release tag and reject unsafe application-only releases.
#
# Usage: classify-release.sh MAJOR.MINOR.PATCH
#
# Run this from a Git checkout containing the target tag and its history. The
# script writes GitHub Actions-compatible key=value outputs to stdout. Errors
# and validation details are written to stderr.

set -euo pipefail

tag="${1:-}"
if ! [[ "$tag" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    echo "release tags must use MAJOR.MINOR.PATCH (got: $tag)" >&2
    exit 1
fi

repo_root="$(git rev-parse --show-toplevel)"
current_train="${tag%.*}"
previous=""
while IFS= read -r candidate; do
    if [[ "$candidate" != "$tag" && "$candidate" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
        previous="$candidate"
        break
    fi
done < <(git -C "$repo_root" tag --merged "$tag" --list '[0-9]*' --sort=-v:refname)

kind="infrastructure"
label="Infrastructure"
if [ -n "$previous" ] && [ "${previous%.*}" = "$current_train" ]; then
    kind="application"
    label="Application"

    # These paths affect host convergence, provider toolchains, or workspace
    # images. They cannot ship as a patch release because patch updates
    # deliberately skip the infrastructure updater.
    protected_changes="$(git -C "$repo_root" diff --name-only "$previous" "$tag" -- \
        infra/install.sh \
        infra/update.sh \
        infra/deploy-app.sh \
        infra/upgrade-workspaces.sh \
        infra/steps \
        infra/lib \
        infra/templates \
        infra/launcher \
        infra/versions.env \
        backend/internal/agent/provisioning \
        backend/internal/agent/antigravity/profile.go \
        backend/internal/agent/claude/assets \
        backend/internal/agent/claude/profile.go \
        backend/internal/agent/codex/provisioning.go \
        backend/internal/agent/kimi/profile.go \
        backend/internal/service/container/image)"
    if [ -n "$protected_changes" ]; then
        echo "patch release $tag changes infrastructure-managed paths:" >&2
        echo "$protected_changes" >&2
        echo "bump the minor (or major) version so installations run the full updater" >&2
        exit 1
    fi
fi

printf 'kind=%s\n' "$kind"
printf 'label=%s\n' "$label"
printf 'previous=%s\n' "$previous"
