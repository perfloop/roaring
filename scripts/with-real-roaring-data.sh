#!/usr/bin/env sh
set -eu

# Keep the documented real-roaring-datasets dependency outside the checkout.
# The commit is pinned so every proof replay receives the same compressed inputs.
gopath=/workspace/deps/real-roaring-gopath
repo="$gopath/src/github.com/RoaringBitmap/real-roaring-datasets"
commit=929d8088817840f43ffaa8592b49373b5a2d43b2

mkdir -p "$(dirname "$repo")"
if [ ! -d "$repo/.git" ]; then
	git init "$repo" >&2
	git -C "$repo" remote add origin https://github.com/RoaringBitmap/real-roaring-datasets.git
fi
if ! git -C "$repo" cat-file -e "$commit^{commit}" 2>/dev/null; then
	git -C "$repo" fetch --depth=1 origin "$commit" >&2
fi
git -C "$repo" checkout --detach "$commit" >&2

exec env GOPATH="$gopath" GOMODCACHE=/workspace/deps/go/pkg/mod "$@"
