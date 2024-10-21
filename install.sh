#!/bin/sh

set -eu

CHANNEL=${CHANNEL:-stable}
VERSION=${VERSION:-current}

os=$(uname -s)
arch=$(uname -m)
OS=${OS:-"${os}"}
ARCH=${ARCH:-"${arch}"}
OS_ARCH=""

BIN=${BIN:-up}

unsupported_arch() {
  local os=$1
  local arch=$2
  echo "Up does not support $os / $arch at this time."
  exit 1
}

case $OS in
  CYGWIN* | MINGW64*)
    if [ $ARCH = "amd64" ]
    then
      OS_ARCH=windows_amd64
      BIN=up.exe
    else
      unsupported_arch $OS $ARCH
    fi
    ;;
  Darwin)
    case $ARCH in
      x86_64|amd64)
        OS_ARCH=darwin_amd64
        ;;
      arm64|aarch64)
        OS_ARCH=darwin_arm64
        ;;
      *)
        unsupported_arch $OS $ARCH
        ;;
    esac
    ;;
  Linux)
    case $ARCH in
      x86_64|amd64)
        OS_ARCH=linux_amd64
        ;;
      arm64|aarch64)
        OS_ARCH=linux_arm64
        ;;
      *)
        unsupported_arch $OS $ARCH
        ;;
    esac
    ;;
  *)
    unsupported_arch $OS $ARCH
    ;;
esac

url="https://cli.upbound.io/${CHANNEL}/${VERSION}/bin/${OS_ARCH}/${BIN}"
if ! curl -sLO ${url}; then
  echo "Failed to download Up. Please make sure version ${VERSION} exists on channel ${CHANNEL}."
  exit 1
fi

if [ $BIN = "up" ]; then
  chmod +x up

  echo "Up downloaded successfully!"
  echo "By proceeding, you are accepting to comply with terms and conditions in https://licenses.upbound.io/upbound-software-license.html"
  echo
  echo "Run the following commands to finish installation:"
  echo
  echo sudo mv up /usr/local/bin/
  echo up version
  echo
  echo "Visit https://upbound.io to get started. 🚀"
  echo "Have a nice day! 👋"
  echo
fi

if [ $BIN = "docker-credential-up" ]; then
  chmod +x docker-credential-up

  echo "Upbound Docker Credential Helper downloaded successfully!"
  echo "By proceeding, you are accepting to comply with terms and conditions in https://licenses.upbound.io/upbound-software-license.html"
  echo
  echo "Run the following commands to finish installation:"
  echo
  echo sudo mv docker-credential-up /usr/local/bin/
  echo docker-credential-up -v
  echo
  echo 'Add "xpkg.upbound.io": "up" to the "credHelpers" section of your Docker config file. 🚀'
  echo "Have a nice day! 👋"
  echo
fi
