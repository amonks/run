#!/usr/bin/env bash

set -euo pipefail

if test -z "$APPLE_DEVELOPER_ID" ||
	test -z "$APPLE_DEVELOPER_TEAM" ||
	test -z "$APPLE_DEVELOPER_PASSWORD" ||
	test -z "$CERTIFICATE_BASE64" ||
	test -z "$CERTIFICATE_PASSWORD" ; then

	echo "env not set"
	exit 1
fi

function log() { echo "$@" 1>&2 ; }

function main() {
	git fetch --tags --force

	rm -rf dist ; mkdir dist
	bdir="$(mktemp -d)"
	pdir="$(mktemp -d)"

	# shellcheck disable=2154
	trap 'code=$? ; echo "FAILURE: $code" ; rm -rf $bdir $pdir ; exit $code' EXIT

	version_name="$(git describe --tags --abbrev=0 HEAD)"
	log "-- Building '$version_name'"

	setup_macos_keychain

	build freebsd arm64
	build freebsd amd64

	build linux   arm64
	build linux   amd64

	build darwin  arm64
	build darwin  amd64
	make_macos_universal_binary

	sign_macos run_darwin_amd64
	sign_macos run_darwin_arm64
	sign_macos run_darwin_universal

	lipod_submission_id="$(notarize_macos run_darwin_universal)"
	amd64_submission_id="$(notarize_macos run_darwin_amd64)"
	arm64_submission_id="$(notarize_macos run_darwin_arm64)"
	wait_for_notarization "$lipod_submission_id"
	wait_for_notarization "$amd64_submission_id"
	wait_for_notarization "$arm64_submission_id"

	create_package freebsd arm64
	create_package freebsd amd64
	create_package linux   arm64
	create_package linux   amd64
	create_package darwin  arm64
	create_package darwin  amd64
	create_package darwin  universal

	create_release
}

# setup_macos_keychain adds $CERTIFICATE_BASE64 to the macOS keychain.
function setup_macos_keychain() {
	log "-- Setting up macos keychain"

	cert_path="$(mktemp -d)/cert.p12"
	keychain_path="$(mktemp -d)/keys.keychain-db"
	keychain_password="$(openssl rand -base64 10)"

	# decode cert to file
        echo -n "$CERTIFICATE_BASE64" | base64 --decode -o "$cert_path"

        # create temporary keychain
        security create-keychain -p "$keychain_password" "$keychain_path"
        security set-keychain-settings -lut 21600 "$keychain_path"
        security unlock-keychain -p "$keychain_password" "$keychain_path"

        # import certificate to keychain
        security import "$cert_path" \
		-P "$CERTIFICATE_PASSWORD" \
		-A \
		-t cert \
		-f pkcs12 \
		-k "$keychain_path"
        security set-key-partition-list \
		-S apple-tool:,apple: \
		-k "$keychain_password" \
		"$keychain_path"
        security list-keychain \
		-d user \
		-s "$keychain_path"
}

# build, given GOOS and GOARCH values, builds a binary file and places it
# in $bdir.
function build() {
	goos="$1"; goarch="$2"
	if test -z "$goos" || test -z "$goarch" ; then
		log "build requires 2 arguments"
		return 1
	fi

	log "-- Building $goos ($goarch)"

	CGO_ENABLED=0 GOOS=$goos GOARCH=$goarch \
		go build \
		-ldflags="-s -w -X 'github.com/amonks/run.Version=${version_name}'" \
		-o "${bdir}/run_${goos}_${goarch}" \
		./cmd/run
}

## make_macos_universal_binary combines $bdir/run_darwin_amd64 and
#$bdir/run_darwin_arm64 into a "fat" universal binary called
#$bdir/run_darwin_universal.
function make_macos_universal_binary() {
	log "-- Making universal macos binary"
	lipo \
		-output "${bdir}/run_darwin_universal" \
		-create "${bdir}/run_darwin_amd64" "${bdir}/run_darwin_arm64"
}

## sign_macos codesigns the given macOS binary.
function sign_macos() {
	binary=$1
	if test -z "$binary" ; then
		log "sign_macos requires 1 argument"
		return 1
	fi

	log "-- Signing $binary"

	codesign \
		--keychain buildagent \
		-s 'Developer ID Application: Andrew Monks (89WR6ARSCL)' \
		--timestamp \
		--options runtime \
		"${bdir}/${binary}"

	return 0
}

# notarize_macos, given a binary file, sends it to Apple for notarization. It
# returns immediately after the submission is submitted, it **does not** wait
# for the submission to complete. It returns the Submission ID, which can be
# used to wait for the submission or check its status.
#
# When a binary is run for the first time on macOS, the operating system
# sends its hash to Apple's server to check whether it appears in their
# notarization database. If it is not, the binary won't run.
function notarize_macos() {
	binary="$1"
	if test -z "$binary" ; then
		log "notarize_macos requires 1 argument"
		return 1
	fi

	log "-- Asking Apple to notarize $binary; this could take some time"

	/usr/bin/ditto -c -k \
		--keepParent \
		"${bdir}/${binary}" \
		"${bdir}/${binary}.zip"

	xcrun notarytool submit \
		--output-format json \
		--apple-id "$APPLE_DEVELOPER_ID" \
		--team-id "$APPLE_DEVELOPER_TEAM" \
		--password "$APPLE_DEVELOPER_PASSWORD" \
		"${bdir}/${binary}.zip" | \
		sed 's/.*"id":"\([-A-z0-9]\{36\}\)".*/\1/'
}

function wait_for_notarization() {
	submission_id="$1"
	if test -z "$submission_id" ; then
		log "wait_for_notarization requires 1 argument"
		return 1
	fi

	log "-- Waiting for notarization submission '$submission_id'"

	xcrun notarytool wait "$submission_id" \
		--apple-id "$APPLE_DEVELOPER_ID" \
		--team-id "$APPLE_DEVELOPER_TEAM" \
		--password "$APPLE_DEVELOPER_PASSWORD"
}

# create_pacakge, given GOOS and GOARCH values, produces a tar.gz archive
# in ./dist. The archive contains the relevant platform's binary, plus
# support files. It then appends the archive's checksum to
# ./dist/checksum.txt.
#
# The relevant binary must already exist in $pdir when create_package is
# called.
function create_package() {
	os="$1"; arch="$2"
	if test -z "$os" || test -z "$arch" ; then
		log "create_package requires 2 arguments"
		return 1
	fi

	package_name="$(package_name "$os" "$arch")"
	log "-- Creating package $package_name"

	binary="${bdir}/run_${os}_${arch}"
	mkdir -p "${pdir}/${package_name}"

	cp -r docs            "${pdir}/${package_name}/"
	cp    CONTRIBUTORS.md "${pdir}/${package_name}/"
	cp    CREDITS.txt     "${pdir}/${package_name}/"
	cp    LICENSE.md      "${pdir}/${package_name}/"
	cp    README.md       "${pdir}/${package_name}/"
	cp    "$binary"       "${pdir}/${package_name}/run"

	distdir="$(pwd)/dist"
	pushd "${pdir}/${package_name}"
	tar \
		--create \
		--gzip \
		--file "${distdir}/${package_name}.tar.gz" \
		. \
		&> /dev/null
	popd

	pushd dist ; shasum "${package_name}.tar.gz" >> checksums.txt ; popd
}

# package_name, given GOOS and GOARCH values, produces the same string
# that this command would on the target platform:
#     echo "run_$(uname -s)_$(uname -m)"
function package_name() {
	os="$1"; arch="$2"
	if test -z "$os" || test -z "$arch" ; then
		log "package_name requires 2 arguments"
		return 1
	fi

	os_name="" ; arch_name=""
	case $os in
		darwin)  os_name="Darwin"  ;;
		freebsd) os_name="FreeBSD" ;;
		linux)   os_name="Linux"   ;;
		*)       log "unsupported os $os" ; exit 1 ;;
	esac

	case $arch in
		arm64)     arch_name="arm64"     ;;
		amd64)     arch_name="amd64"     ;;
		universal) arch_name="universal" ;;
		*)         log "unsupported arch $arch" ; exit 1 ;;
	esac

	if test "$os" = "darwin" && test "$arch" = "amd64" ; then
		arch_name="x86_64"
	fi

	echo "run_${os_name}_${arch_name}"
}

# create_release_notes produces markdown including a changelog (heuristic:
# since the last git tag) and instructions for determining the right
# binary for a given platform.
function create_release_notes() {
	last_release="$(git describe --tags --abbrev=0 HEAD~)"
	echo "## Changelog"
	git log "$last_release..HEAD" --oneline --decorate=no | sort | awk '{ print "- " $0 }'
	echo ""
	if git log "$last_release..HEAD" | grep -q BREAKING ; then
		echo "## Breaking Changes"
		git log "$last_release..HEAD" | \
			grep BREAKING | \
			sed 's/^[[:space:]]*BREAKING CHANGE: //g' | \
			awk '{ print "- " $0 }'
		echo ""
	fi
	echo "> [!TIP]"
	echo "> You can find the correct asset for your system with the following command:"
	# shellcheck disable=2016
	echo '> `echo "run_$(uname -s)_$(uname -m).tar.gz"`'
}

# create_release uses the Github API to create the release and upload the
# release artifacts.
function create_release() {
	log "-- Uploading release to Github"
	create_release_notes | gh release create \
		--repo="${GITHUB_REPOSITORY:-amonks/run}" \
		--notes-file=- \
		"$version_name" \
		dist/*
}

main
