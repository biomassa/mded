#!/bin/sh
# release.sh makes an mded release: it runs the tests, builds the binaries for Linux
# and macOS (amd64 and arm64), makes the Git tag, and publishes a GitHub release with the
# notes of the version from CHANGELOG.md.
#
#   scripts/release.sh v0.1.0            make and publish the release
#   scripts/release.sh --dry-run v0.1.0  build and show the notes, but do not tag or publish
#
# Before a release, move the [Unreleased] notes of CHANGELOG.md under "## [0.1.0] - date"
# and commit.
#
# The checks before the build: go vet, the tests, the tests with the race detector, and the
# benchmarks. The tests and the benchmarks run with a memory limit of 3 GB and no swap
# (systemd-run), so that a leak fails the run and does not stop the computer. The benchmarks
# of HEAD are compared with the benchmarks of the last tag (benchstat). A benchmark that is
# more than 20% slower, with a significant difference, stops the release. If the last tag
# has no benchmarks, or they do not compile there, the script shows a warning and goes on.
set -eu

dry=false
if [ "${1:-}" = "--dry-run" ]; then
	dry=true
	shift
fi
tag=${1:?"usage: scripts/release.sh [--dry-run] vX.Y.Z"}
case "$tag" in
v[0-9]*.[0-9]*.[0-9]*) ;;
*)
	echo "the version must look like v0.1.0" >&2
	exit 1
	;;
esac
ver=${tag#v}
cd "$(dirname "$0")/.."

fail() {
	echo "$1" >&2
	exit 1
}
if ! $dry; then
	[ -z "$(git status --porcelain)" ] || fail "commit or stash the changes first"
	[ "$(git branch --show-current)" = main ] || fail "make a release from the main branch"
	! git rev-parse -q --verify "refs/tags/$tag" >/dev/null || fail "the tag $tag exists already"
fi
grep -q "^## \[$ver\] - " CHANGELOG.md || fail "CHANGELOG.md has no section \"## [$ver] - date\""

# benchstat is pinned, so that the compare does not change between releases.
benchstat="go run golang.org/x/perf/cmd/benchstat@v0.0.0-20261009192801-be2c69fb417e"
benchLimit=20 # percent

# capped runs a command with a memory limit of 3 GB and no swap, and a time limit.
capped() {
	if command -v systemd-run >/dev/null; then
		systemd-run --user --scope -q -p MemoryMax=3G -p MemorySwapMax=0 timeout -k 5 1200 "$@"
	else
		echo "warning: no systemd-run, the command runs without a memory limit: $*" >&2
		"$@"
	fi
}

# bench runs the benchmarks of the module in the directory $1 and writes them to $2.
bench() {
	(cd "$1" && capped go test ./... -run '^$' -bench . -benchtime 1s -count 6) >"$2"
}

go vet ./...
capped go test ./...
capped go test -race ./...

work=$(mktemp -d)
trap 'git worktree remove --force "$work/old" 2>/dev/null || true; rm -rf "$work"' EXIT
bench . "$work/new.txt"
prev=$(git describe --tags --abbrev=0 2>/dev/null || true)
if [ -z "$prev" ]; then
	echo "warning: no earlier tag, no benchmark compare" >&2
	cat "$work/new.txt"
else
	git worktree add -q --detach "$work/old" "$prev"
	# The benchmark files of HEAD measure the code of the last tag too.
	git ls-files --cached --others --exclude-standard '*bench_test.go' |
		while read -r f; do cp "$f" "$work/old/$f"; done
	if ! bench "$work/old" "$work/old.txt" || ! grep -q '^Benchmark' "$work/old.txt"; then
		echo "warning: the benchmarks do not run on $prev, no compare" >&2
		cat "$work/new.txt"
	else
		$benchstat -format csv "$work/old.txt" "$work/new.txt" >"$work/cmp.csv"
		$benchstat "$work/old.txt" "$work/new.txt"
		# The first table is sec/op. A row with a significant change has "+12.34%" in
		# column 6; benchstat writes "~" for a change that is not significant.
		slow=$(awk -F, -v lim="$benchLimit" '
			/sec\/op/ { on = 1; next }
			on && $0 == "" { exit }
			on && $6 ~ /^\+/ { v = substr($6, 2); sub(/%/, "", v); if (v + 0 > lim) print $1 " " $6 }
		' "$work/cmp.csv")
		[ -z "$slow" ] || fail "benchmarks more than $benchLimit% slower than $prev:
$slow"
	fi
fi

rm -rf dist
mkdir dist
for target in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64; do
	os=${target%/*}
	arch=${target#*/}
	name=mded_${ver}_${os}_${arch}
	mkdir "dist/$name"
	CGO_ENABLED=0 GOOS=$os GOARCH=$arch go build -trimpath \
		-ldflags "-s -w -X github.com/biomassa/mded/internal/version.Version=$ver" \
		-o "dist/$name/mded" ./cmd/mded
	cp README.md CHANGELOG.md "dist/$name/"
	[ ! -f LICENSE ] || cp LICENSE "dist/$name/"
	tar -C dist -czf "dist/$name.tar.gz" "$name"
	rm -r "dist/$name"
done
if command -v sha256sum >/dev/null; then
	(cd dist && sha256sum ./*.tar.gz >checksums.txt)
else
	(cd dist && shasum -a 256 ./*.tar.gz >checksums.txt)
fi

# The release notes are the section of this version in CHANGELOG.md, without its heading.
awk -v v="$ver" '
	index($0, "## [" v "]") == 1 { on = 1; next }
	on && /^## \[/ { exit }
	on && /^\[[^]]+\]: / { exit }
	on { print }
' CHANGELOG.md >dist/notes.md

ls -l dist
if $dry; then
	echo "dry run: the release notes are in dist/notes.md. No tag, no release."
	exit 0
fi
git tag -a "$tag" -m "mded $ver"
git push origin main "$tag"
gh release create "$tag" dist/*.tar.gz dist/checksums.txt --title "mded $ver" --notes-file dist/notes.md
echo "released $tag"
