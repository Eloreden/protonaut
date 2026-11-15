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
#
# WebKitGTK runs the page in helper processes (WebKitWebProcess,
# WebKitNetworkProcess, ...) that it spawns from an absolute path compiled
# into libwebkit2gtk (e.g. /usr/lib/x86_64-linux-gnu/webkit2gtk-4.1).
# linuxdeploy bundles the library but not the helpers, so on any distro with
# a different layout the app aborts at startup. We bundle the helpers, rewrite
# that path in the bundled library to a same-length relative one ("/usr" ->
# "././") and cd into $APPDIR/usr from an AppRun hook so it resolves.
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

echo "==> Building protonaut (webkit2gtk-4.1, trimpath, version=$VERSION)"
(cd "$ROOT_DIR" && wails build -tags webkit2_41 -trimpath \
    -ldflags "-w -s -X main.Version=$VERSION")

echo "==> Assembling AppDir"
rm -rf "$APPDIR"
mkdir -p "$APPDIR/usr/bin" "$APPDIR/usr/share/applications" \
    "$APPDIR/usr/share/icons/hicolor/512x512/apps"
cp "$ROOT_DIR/build/bin/Protonaut" "$APPDIR/usr/bin/protonaut"
cp "$ROOT_DIR/packaging/linux/protonaut.desktop" "$APPDIR/usr/share/applications/protonaut.desktop"
convert "$ROOT_DIR/build/appicon.png" -resize 512x512 \
    "$APPDIR/usr/share/icons/hicolor/512x512/apps/protonaut.png"

echo "==> Bundling WebKitGTK helper processes"
WEBKIT_HELPERS="$(pkg-config --variable=libdir webkit2gtk-4.1)/webkit2gtk-4.1"
case "$WEBKIT_HELPERS" in
    /usr/*) ;;
    *) echo "unexpected webkit2gtk-4.1 libdir: $WEBKIT_HELPERS" >&2; exit 1 ;;
esac
WEBKIT_HELPERS_IN_APPDIR="$APPDIR/usr/${WEBKIT_HELPERS#/usr/}"
mkdir -p "$WEBKIT_HELPERS_IN_APPDIR/injected-bundle"
for helper in WebKitWebProcess WebKitNetworkProcess WebKitGPUProcess; do
    [ -e "$WEBKIT_HELPERS/$helper" ] && cp -L "$WEBKIT_HELPERS/$helper" "$WEBKIT_HELPERS_IN_APPDIR/"
done
cp -L "$WEBKIT_HELPERS/injected-bundle/libwebkit2gtkinjectedbundle.so" "$WEBKIT_HELPERS_IN_APPDIR/injected-bundle/"
for f in "$WEBKIT_HELPERS_IN_APPDIR"/* "$WEBKIT_HELPERS_IN_APPDIR"/injected-bundle/*.so; do
    [ -f "$f" ] || continue
    # the helpers must load the bundled libwebkit2gtk & co. from usr/lib, never the host's
    patchelf --set-rpath "\$ORIGIN/$(realpath -m --relative-to="$(dirname "$f")" "$APPDIR/usr/lib")" "$f"
done

mkdir -p "$APPDIR/apprun-hooks"
cat > "$APPDIR/apprun-hooks/webkit-helpers.sh" <<'HOOK'
# libwebkit2gtk was patched to look for its helper processes under
# ././/lib/.../webkit2gtk-4.1, relative to $APPDIR/usr: run from there.
cd "${APPDIR:-$this_dir}/usr"
HOOK

echo "==> Running linuxdeploy (bundles GTK/WebKit runtime deps)"
export PATH="$TOOLS_DIR:$PATH"
export DEPLOY_GTK_VERSION=3
export APPIMAGE_EXTRACT_AND_RUN=1
export VERSION

# Embed update information so AppImageUpdate/AppImageLauncher can update the
# installed AppImage in place (no duplicate files) even as the versioned
# filename changes. The '*' matches the version; 'latest' resolves the newest
# GitHub Release. appimagetool also emits a matching .zsync file next to it.
export UPDATE_INFORMATION="${UPDATE_INFORMATION:-gh-releases-zsync|Eloreden|protonaut|latest|Protonaut-*-x86_64.AppImage}"

(cd "$BUILD_DIR" && "$LINUXDEPLOY_EXTRACTED/AppRun" \
    --appdir "$APPDIR" \
    --executable "$APPDIR/usr/bin/protonaut" \
    --desktop-file "$APPDIR/usr/share/applications/protonaut.desktop" \
    --icon-file "$APPDIR/usr/share/icons/hicolor/512x512/apps/protonaut.png" \
    --deploy-deps-only "$WEBKIT_HELPERS_IN_APPDIR" \
    --deploy-deps-only "$WEBKIT_HELPERS_IN_APPDIR/injected-bundle" \
    --plugin gtk)

echo "==> Pointing the bundled libwebkit2gtk at the bundled helpers"
python3 - "$APPDIR/usr/lib/libwebkit2gtk-4.1.so.0" "$WEBKIT_HELPERS" <<'PY'
import sys
lib, helpers = sys.argv[1], sys.argv[2].encode()
relative = b"././" + helpers[len(b"/usr"):]          # same length, resolved from $APPDIR/usr
assert len(relative) == len(helpers)
data = open(lib, "rb").read()
count = data.count(helpers)
if count == 0:
    sys.exit(f"{helpers.decode()} not found in {lib}")
open(lib, "wb").write(data.replace(helpers, relative))
print(f"patched {count} occurrence(s)")
PY

echo "==> Packaging AppImage"
(cd "$BUILD_DIR" && "$LINUXDEPLOY_EXTRACTED/usr/bin/linuxdeploy-plugin-appimage" --appdir "$APPDIR")

echo "==> Done: $BUILD_DIR/Protonaut-$VERSION-x86_64.AppImage (+ .zsync)"
