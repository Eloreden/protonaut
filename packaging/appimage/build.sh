#!/usr/bin/env bash
# Builds a self-contained Protonaut AppImage.
#
# Downloads linuxdeploy + its GTK plugin + appimagetool into
# packaging/appimage/tools/ (cached across runs), builds the app with the
# same flags used by the AUR package, assembles an AppDir and packages it.
#
# The bundled `strip` shipped inside linuxdeploy's continuous build predates
# the RELR relocation format (SHT_RELR) used by recent toolchains, and
# aborts on any library built with one. We swap it for the system `strip`,
# which is backwards compatible, before running linuxdeploy.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"
TOOLS_DIR="$SCRIPT_DIR/tools"
BUILD_DIR="$SCRIPT_DIR/build"
APPDIR="$BUILD_DIR/AppDir"

VERSION="${VERSION:-$(git -C "$ROOT_DIR" rev-parse --short HEAD)}"

mkdir -p "$TOOLS_DIR"

fetch() {
    local url="$1" out="$2"
    [ -f "$out" ] || curl -sL -o "$out" "$url"
}

fetch "https://github.com/linuxdeploy/linuxdeploy/releases/download/continuous/linuxdeploy-x86_64.AppImage" \
    "$TOOLS_DIR/linuxdeploy-x86_64.AppImage"
fetch "https://raw.githubusercontent.com/linuxdeploy/linuxdeploy-plugin-gtk/master/linuxdeploy-plugin-gtk.sh" \
    "$TOOLS_DIR/linuxdeploy-plugin-gtk.sh"
chmod +x "$TOOLS_DIR/linuxdeploy-x86_64.AppImage" "$TOOLS_DIR/linuxdeploy-plugin-gtk.sh"

# Extract linuxdeploy once and swap its bundled strip for the system one.
LINUXDEPLOY_EXTRACTED="$TOOLS_DIR/linuxdeploy-extracted"
if [ ! -x "$LINUXDEPLOY_EXTRACTED/AppRun" ]; then
    rm -rf "$LINUXDEPLOY_EXTRACTED"
    mkdir -p "$LINUXDEPLOY_EXTRACTED"
    (cd "$LINUXDEPLOY_EXTRACTED" && "$TOOLS_DIR/linuxdeploy-x86_64.AppImage" --appimage-extract >/dev/null)
    mv "$LINUXDEPLOY_EXTRACTED/squashfs-root" "$LINUXDEPLOY_EXTRACTED.tmp"
    rm -rf "$LINUXDEPLOY_EXTRACTED"
    mv "$LINUXDEPLOY_EXTRACTED.tmp" "$LINUXDEPLOY_EXTRACTED"
    ln -sf "$(command -v strip)" "$LINUXDEPLOY_EXTRACTED/usr/bin/strip"
fi

echo "==> Building protonaut (webkit2gtk-4.1, trimpath)"
(cd "$ROOT_DIR" && wails build -tags webkit2_41 -trimpath -ldflags "-w -s")

echo "==> Assembling AppDir"
rm -rf "$APPDIR"
mkdir -p "$APPDIR/usr/bin" "$APPDIR/usr/share/applications" \
    "$APPDIR/usr/share/icons/hicolor/512x512/apps"
cp "$ROOT_DIR/build/bin/Protonaut" "$APPDIR/usr/bin/protonaut"
cp "$ROOT_DIR/packaging/linux/protonaut.desktop" "$APPDIR/usr/share/applications/protonaut.desktop"
magick "$ROOT_DIR/build/appicon.png" -resize 512x512 \
    "$APPDIR/usr/share/icons/hicolor/512x512/apps/protonaut.png"

echo "==> Running linuxdeploy (bundles GTK/WebKit runtime deps)"
export PATH="$TOOLS_DIR:$PATH"
export DEPLOY_GTK_VERSION=3
export APPIMAGE_EXTRACT_AND_RUN=1
export VERSION

(cd "$BUILD_DIR" && "$LINUXDEPLOY_EXTRACTED/AppRun" \
    --appdir "$APPDIR" \
    --executable "$APPDIR/usr/bin/protonaut" \
    --desktop-file "$APPDIR/usr/share/applications/protonaut.desktop" \
    --icon-file "$APPDIR/usr/share/icons/hicolor/512x512/apps/protonaut.png" \
    --plugin gtk \
    --output appimage)

echo "==> Done: $BUILD_DIR/Protonaut-$VERSION-x86_64.AppImage"
