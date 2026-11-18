#!/bin/bash
# Baut Tri-Vertical.app für macOS. Auf einem Mac ausführen (siehe MAC-LIESMICH.txt).
# Voraussetzungen: Xcode-Befehlszeilenwerkzeuge (xcode-select --install), Go ab 1.24, git.
# Aufruf:  ./baue_mac.sh            (für den Mac, auf dem gebaut wird)
#          ARCH=amd64 ./baue_mac.sh (Intel-Build erzwingen; Standard = Architektur dieses Macs)
set -e
cd "$(dirname "$0")"
VER=$(cat ../setup-bausatz/hauptprogramm/version.txt | tr -d '[:space:]')
ARCH=${ARCH:-$(go env GOARCH)}
export GOFLAGS=-mod=mod GOSUMDB=off CGO_ENABLED=1 GOOS=darwin GOARCH=$ARCH
export MACOSX_DEPLOYMENT_TARGET=11.0
case $ARCH in arm64) CARCH=arm64;; amd64) CARCH=x86_64;; esac
export CGO_CFLAGS="-arch $CARCH -DLIBDE265_STATIC_BUILD" CGO_CXXFLAGS="-arch $CARCH -DLIBDE265_STATIC_BUILD" CGO_LDFLAGS="-arch $CARCH"

# 1. LibRaw (RAW-Entwicklung) einmalig als statische Bibliothek bauen
if [ ! -f roh/lib/darwin/libraw.a ]; then
  echo "== Baue LibRaw …"
  T=$(mktemp -d)
  git clone https://github.com/LibRaw/LibRaw.git "$T/src"
  (cd "$T/src" && git checkout "$(tr -d '[:space:]' < "$OLDPWD/roh/quelle/LIBRAW-COMMIT.txt")")
  mkdir -p roh/lib/darwin
  bash roh/quelle/bauelr.sh "$T/src" "$PWD/roh/lib/darwin" "clang++ -arch $CARCH" ar
  rm -rf "$T"
fi

# 2. Programm bauen
echo "== Baue Tri-Vertical $VER ($ARCH) …"
mkdir -p ../mac-build
go build -ldflags '-s -w' -o ../mac-build/tri-vertical ./cmd/vertical3mac

# 3. .app-Paket zusammenstellen
APP=../mac-build/Tri-Vertical.app
rm -rf "$APP"; mkdir -p "$APP/Contents/MacOS" "$APP/Contents/Resources"
cp ../mac-build/tri-vertical "$APP/Contents/MacOS/Tri-Vertical"
ICNS=""
if command -v iconutil >/dev/null && command -v sips >/dev/null && [ -f icon.png ]; then
  I=$(mktemp -d)/Tri-Vertical.iconset; mkdir -p "$I"
  for s in 16 32 64 128 256 512; do
    sips -z $s $s icon.png --out "$I/icon_${s}x${s}.png" >/dev/null
    sips -z $((s*2)) $((s*2)) icon.png --out "$I/icon_${s}x${s}@2x.png" >/dev/null
  done
  iconutil -c icns "$I" -o "$APP/Contents/Resources/Tri-Vertical.icns" && ICNS=Tri-Vertical
fi
cat > "$APP/Contents/Info.plist" <<PL
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
 <key>CFBundleName</key><string>Tri-Vertical</string>
 <key>CFBundleDisplayName</key><string>Tri-Vertical</string>
 <key>CFBundleIdentifier</key><string>de.tri-vertical.mac</string>
 <key>CFBundleVersion</key><string>$VER</string>
 <key>CFBundleShortVersionString</key><string>$VER</string>
 <key>CFBundleExecutable</key><string>Tri-Vertical</string>
 <key>CFBundlePackageType</key><string>APPL</string>
 <key>CFBundleIconFile</key><string>$ICNS</string>
 <key>LSMinimumSystemVersion</key><string>11.0</string>
 <key>NSHighResolutionCapable</key><true/>
 <key>CFBundleDocumentTypes</key><array><dict>
  <key>CFBundleTypeName</key><string>Bild</string>
  <key>CFBundleTypeRole</key><string>Editor</string>
  <key>LSItemContentTypes</key><array><string>public.image</string><string>public.camera-raw-image</string></array>
 </dict></array>
</dict></plist>
PL
# ohne Entwicklerkonto: ad-hoc signieren (sonst verweigert macOS auf Apple-Chips den Start)
codesign --force --deep --sign - "$APP" 2>/dev/null || true
echo "Fertig: $(cd ../mac-build && pwd)/Tri-Vertical.app"
