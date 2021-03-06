package templates

const BuildTemplate = `
#!/bin/bash

if [ $# -ne 1 ]; then
  echo "Need to specify device name as argument"
  exit 1
fi

# check if supported device
DEVICE=$1
if [ "$DEVICE" == 'sailfish' ] || [ "$DEVICE" == 'marlin' ] || [ "$DEVICE" == 'walleye' ] || [ "$DEVICE" == 'taimen' ]; then
  echo "Supported device $DEVICE - continuing build"
else 
  echo "Unsupported device $DEVICE"
  exit 1
fi

# set region
REGION=<% .Region %>
export AWS_DEFAULT_REGION=${REGION}

# stack name
STACK_NAME=<% .Name %>

# version of stack running
STACK_VERSION=<% .Version %>

# prevent default action of shutting down on exit
PREVENT_SHUTDOWN=<% .PreventShutdown %>

# force build even if no new versions exist of components
FORCE_BUILD=<% .Force %>

# skip chromium build if there is an existing build
SKIP_CHROMIUM_BUILD=<% .SkipChromiumBuild %>

# pin to specific version of android
ANDROID_VERSION="9.0"

# build type (user or userdebug)
BUILD_TYPE="user"

# build channel (stable or beta)
BUILD_CHANNEL="stable"

# user customizable things
REPO_PATCHES=<% .RepoPatches %>
REPO_PREBUILTS=<% .RepoPrebuilts %>
HOSTS_FILE=<% .HostsFile %>

# aws settings
AWS_KEYS_BUCKET="${STACK_NAME}-keys"
AWS_RELEASE_BUCKET="${STACK_NAME}-release"
AWS_LOGS_BUCKET="${STACK_NAME}-logs"
AWS_SNS_ARN=$(aws --region ${REGION} sns list-topics --query 'Topics[0].TopicArn' --output text | cut -d":" -f1,2,3,4,5)":${STACK_NAME}"
INSTANCE_TYPE=$(curl -s http://169.254.169.254/latest/meta-data/instance-type)
INSTANCE_REGION=$(curl -s http://169.254.169.254/latest/dynamic/instance-identity/document | awk -F\" '/region/ {print $4}')
INSTANCE_IP=$(curl -s http://169.254.169.254/latest/meta-data/public-ipv4)

# build settings
SECONDS=0
BUILD_TARGET="release aosp_${DEVICE} ${BUILD_TYPE}"
RELEASE_URL="https://${AWS_RELEASE_BUCKET}.s3.amazonaws.com"
RELEASE_CHANNEL="${DEVICE}-${BUILD_CHANNEL}"
CHROME_CHANNEL="stable"
BUILD_DATE=$(date +%Y.%m.%d.%H)
BUILD_TIMESTAMP=$(date +%s)
BUILD_DIR="$HOME/rattlesnake-os"
CERTIFICATE_SUBJECT='/CN=RattlesnakeOS'
OFFICIAL_FDROID_KEY="43238d512c1e5eb2d6569f4a3afbf5523418b82e0a3ed1552770abb9a9c9ccab"
MARLIN_KERNEL_SOURCE_DIR="${HOME}/kernel/google/marlin"

# urls
ANDROID_SDK_URL="https://dl.google.com/android/repository/sdk-tools-linux-4333796.zip"
MANIFEST_URL="https://android.googlesource.com/platform/manifest"
CHROME_URL_LATEST="https://omahaproxy.appspot.com/all.json"
STACK_URL_LATEST="https://api.github.com/repos/dan-v/rattlesnakeos-stack/releases/latest"
FDROID_CLIENT_URL_LATEST="https://gitlab.com/api/v4/projects/36189/repository/tags"
FDROID_PRIV_EXT_URL_LATEST="https://gitlab.com/api/v4/projects/1481578/repository/tags"
KERNEL_SOURCE_URL="https://android.googlesource.com/kernel/msm"

STACK_UPDATE_MESSAGE=
LATEST_STACK_VERSION=
LATEST_CHROMIUM=
FDROID_CLIENT_VERSION=
FDROID_PRIV_EXT_VERSION=
AOSP_BUILD=
AOSP_BRANCH=
get_latest_versions() {
  log_header ${FUNCNAME}

  sudo apt-get -y install jq
  
  # check if running latest stack
  LATEST_STACK_VERSION=$(curl --fail -s "$STACK_URL_LATEST" | jq -r '.name')
  if [ -z "$LATEST_STACK_VERSION" ]; then
    aws_notify_simple "ERROR: Unable to get latest rattlesnakeos-stack version details. Stopping build."
    exit 1
  elif [ "$LATEST_STACK_VERSION" == "$STACK_VERSION" ]; then
    echo "Running the latest rattlesnakeos-stack version $LATEST_STACK_VERSION"
  else
    STACK_UPDATE_MESSAGE="WARNING: you should upgrade to the latest version: ${LATEST_STACK_VERSION}"
  fi
  
  # check for latest stable chromium version
  LATEST_CHROMIUM=$(curl --fail -s "$CHROME_URL_LATEST" | jq -r '.[] | select(.os == "android") | .versions[] | select(.channel == "'$CHROME_CHANNEL'") | .current_version')
  if [ -z "$LATEST_CHROMIUM" ]; then
    aws_notify_simple "ERROR: Unable to get latest Chromium version details. Stopping build."
    exit 1
  fi
  
  # fdroid - get latest non alpha tags from gitlab
  FDROID_CLIENT_VERSION=$(curl --fail -s "$FDROID_CLIENT_URL_LATEST" | jq -r '[.[] | select(.name | test("^[0-9]+\\.[0-9]+")) | select(.name | contains("alpha") | not) | select(.name | contains("ota") | not)][0] | .name')
  if [ -z "$FDROID_CLIENT_VERSION" ]; then
    aws_notify_simple "ERROR: Unable to get latest F-Droid version details. Stopping build."
    exit 1
  fi
  FDROID_PRIV_EXT_VERSION=$(curl --fail -s "$FDROID_PRIV_EXT_URL_LATEST" | jq -r '[.[] | select(.name | test("^[0-9]+\\.[0-9]+")) | select(.name | contains("alpha") | not) | select(.name | contains("ota") | not)][0] | .name')
  if [ -z "$FDROID_PRIV_EXT_VERSION" ]; then
    aws_notify_simple "ERROR: Unable to get latest F-Droid privilege extension version details. Stopping build."
    exit 1
  fi
  
  # attempt to automatically pick latest build version and branch. note this is likely to break with any page redesign. should also add some validation here.
  AOSP_BUILD=$(curl --fail -s https://developers.google.com/android/images | grep -A1 "${DEVICE}" | egrep '[a-zA-Z]+ [0-9]{4}\)' | grep "${ANDROID_VERSION}" | tail -1 | cut -d"(" -f2 | cut -d"," -f1)
  if [ -z "$AOSP_BUILD" ]; then
    aws_notify_simple "ERROR: Unable to get latest AOSP build information. Stopping build. This lookup is pretty fragile and can break on any page redesign of https://developers.google.com/android/images"
    exit 1
  fi
  AOSP_BRANCH=$(curl --fail -s https://source.android.com/setup/start/build-numbers | grep -A1 "${AOSP_BUILD}" | tail -1 | cut -f2 -d">"|cut -f1 -d"<")
  if [ -z "$AOSP_BRANCH" ]; then
    aws_notify_simple "ERROR: Unable to get latest AOSP branch information. Stopping build. This can happen if https://source.android.com/setup/start/build-numbers hasn't been updated yet with newly released factory images."
    exit 1
  fi
}

check_for_new_versions() {
  log_header ${FUNCNAME}

  echo "Checking if any new versions of software exist"
  needs_update=false

  # check stack version
  existing_stack_version=$(aws s3 cp "s3://${AWS_RELEASE_BUCKET}/rattlesnakeos-stack/revision" - || true)
  if [ "$existing_stack_version" == "$STACK_VERSION" ]; then
    echo "Stack version ($existing_stack_version) is up to date"
  else
    echo "Last successful build (if there was one) is not with latest stack version ${STACK_VERSION}"
    needs_update=true
  fi

  # check aosp
  existing_aosp_build=$(aws s3 cp "s3://${AWS_RELEASE_BUCKET}/${DEVICE}-vendor" - || true)
  if [ "$existing_aosp_build" == "$AOSP_BUILD" ]; then
    echo "AOSP build ($existing_aosp_build) is up to date"
  else
    echo "AOSP needs to be updated to ${AOSP_BUILD}"
    needs_update=true
  fi

  # check chromium
  if [ "$SKIP_CHROMIUM_BUILD" = false ]; then
    existing_chromium=$(aws s3 cp "s3://${AWS_RELEASE_BUCKET}/chromium/revision" - || true)
    if [ "$existing_chromium" == "$LATEST_CHROMIUM" ]; then
      echo "Chromium build ($existing_chromium) is up to date"
    else
      echo "Chromium needs to be updated to ${LATEST_CHROMIUM}"
      needs_update=true
    fi
  else 
    echo "Skipping Chromium version check as SKIP_CHROMIUM_BUILD=true"
  fi

  # check fdroid
  existing_fdroid_client=$(aws s3 cp "s3://${AWS_RELEASE_BUCKET}/fdroid/revision" - || true)
  if [ "$existing_fdroid_client" == "$FDROID_CLIENT_VERSION" ]; then
    echo "F-Droid build ($existing_fdroid_client) is up to date"
  else
    echo "F-Droid needs to be updated to ${FDROID_CLIENT_VERSION}"
    needs_update=true
  fi

  # check fdroid priv extension
  existing_fdroid_priv_version=$(aws s3 cp "s3://${AWS_RELEASE_BUCKET}/fdroid-priv/revision" - || true)
  if [ "$existing_fdroid_priv_version" == "$FDROID_PRIV_EXT_VERSION" ]; then
    echo "F-Droid privilege extension build ($existing_fdroid_priv_version) is up to date"
  else
    echo "F-Droid privilege extensions needs to be updated to ${FDROID_PRIV_EXT_VERSION}"
    needs_update=true
  fi

  if [ "$needs_update" = true ]; then
    echo "New build is required"
  else 
    if [ "$FORCE_BUILD" = true ]; then
      echo "No build is required, but FORCE_BUILD=true"
    else
      aws_notify "RattlesnakeOS build not required as all components are already up to date."
      exit 0
    fi
  fi
}

full_run() {
  log_header ${FUNCNAME}

  get_latest_versions
  check_for_new_versions
  aws_notify "RattlesnakeOS Build STARTED"
  setup_env
  check_chromium
  aosp_repo_init
  aosp_repo_modifications
  aosp_repo_sync
  setup_vendor
  aws_import_keys
  apply_patches
  # only marlin and sailfish need kernel rebuilt so that verity_key is included
  if [ "${DEVICE}" == "marlin" ] || [ "${DEVICE}" == "sailfish" ]; then
    rebuild_marlin_kernel
  fi
  build_aosp
  release "${DEVICE}"
  aws_upload
  checkpoint_versions
  aws_notify "RattlesnakeOS Build SUCCESS"
}

setup_env() {
  log_header ${FUNCNAME}

  # setup build dir
  mkdir -p "$BUILD_DIR"

  # install required packages
  sudo apt-get update
  sudo apt-get -y install repo gperf jq openjdk-8-jdk git-core gnupg flex bison build-essential zip curl zlib1g-dev gcc-multilib g++-multilib libc6-dev-i386 lib32ncurses5-dev x11proto-core-dev libx11-dev lib32z-dev ccache libgl1-mesa-dev libxml2-utils xsltproc unzip python-networkx liblz4-tool pxz
  sudo apt-get -y build-dep "linux-image-$(uname --kernel-release)"

  # setup android sdk (required for fdroid build)
  if [ ! -f "${HOME}/sdk/tools/bin/sdkmanager" ]; then
    mkdir -p ${HOME}/sdk
    cd ${HOME}/sdk
    retry wget ${ANDROID_SDK_URL} -O sdk-tools.zip
    unzip sdk-tools.zip
    yes | ./tools/bin/sdkmanager --licenses
    ./tools/android update sdk -u --use-sdk-wrapper
  fi

  # setup git
  git config --get --global user.name || git config --global user.name 'unknown'
  git config --get --global user.email || git config --global user.email 'unknown@localhost'
  git config --global color.ui true
}

check_chromium() {
  log_header ${FUNCNAME}

  current=$(aws s3 cp "s3://${AWS_RELEASE_BUCKET}/chromium/revision" - || true)
  log "Chromium current: $current"

  if [ "$SKIP_CHROMIUM_BUILD" = true ]; then
    if [ -z "$current" ]; then
      log "Can't skip Chromium build as requested as Chromium hasn't been built yet previously"
    else
      log "Skipping Chromium build as requested"
      aws s3 cp "s3://${AWS_RELEASE_BUCKET}/chromium/MonochromePublic.apk" ${BUILD_DIR}/external/chromium/prebuilt/arm64/
      return
    fi
  fi 

  log "Chromium latest: $LATEST_CHROMIUM"
  if [ "$LATEST_CHROMIUM" == "$current" ]; then
    log "Chromium latest ($LATEST_CHROMIUM) matches current ($current) - just copying s3 chromium artifact"
    aws s3 cp "s3://${AWS_RELEASE_BUCKET}/chromium/MonochromePublic.apk" ${BUILD_DIR}/external/chromium/prebuilt/arm64/
  else
    log "Building chromium $LATEST_CHROMIUM"
    build_chromium $LATEST_CHROMIUM
  fi
  rm -rf $HOME/chromium
}

build_chromium() {
  log_header ${FUNCNAME}

  CHROMIUM_REVISION=$1
  DEFAULT_VERSION=$(echo $CHROMIUM_REVISION | awk -F"." '{ printf "%s%03d52\n",$3,$4}')

  # depot tools setup
  if [ ! -d "$HOME/depot_tools" ]; then
    retry git clone https://chromium.googlesource.com/chromium/tools/depot_tools.git $HOME/depot_tools
  fi
  export PATH="$PATH:$HOME/depot_tools"

  # fetch chromium 
  mkdir -p $HOME/chromium
  cd $HOME/chromium
  fetch --nohooks android
  cd src

  # checkout specific revision
  git checkout "$CHROMIUM_REVISION" -f

  # install dependencies
  echo ttf-mscorefonts-installer msttcorefonts/accepted-mscorefonts-eula select true | sudo debconf-set-selections
  sudo ./build/install-build-deps-android.sh

  # run gclient sync (runhooks will run as part of this)
  yes | gclient sync --with_branch_heads --jobs 32 -RDf

  # cleanup any files in tree not part of this revision
  git clean -dff

  # reset any modifications
  git checkout -- .

  # generate configuration
  mkdir -p out/Default
  cat <<EOF > out/Default/args.gn
target_os = "android"
target_cpu = "arm64"
is_debug = false

is_official_build = true
is_component_build = false
symbol_level = 0

ffmpeg_branding = "Chrome"
proprietary_codecs = true

android_channel = "stable"
android_default_version_name = "$CHROMIUM_REVISION"
android_default_version_code = "$DEFAULT_VERSION"
EOF
  gn gen out/Default

  # build chromium monochrome_public target
  autoninja -C out/Default/ monochrome_public_apk

  # copy to build tree
  mkdir -p ${BUILD_DIR}/external/chromium/prebuilt/arm64
  cp out/Default/apks/MonochromePublic.apk ${BUILD_DIR}/external/chromium/prebuilt/arm64/

  # upload to s3 for future builds
  aws s3 cp "${BUILD_DIR}/external/chromium/prebuilt/arm64/MonochromePublic.apk" "s3://${AWS_RELEASE_BUCKET}/chromium/MonochromePublic.apk"
  echo "${CHROMIUM_REVISION}" | aws s3 cp - "s3://${AWS_RELEASE_BUCKET}/chromium/revision"
}

aosp_repo_init() {
  log_header ${FUNCNAME}
  cd "${BUILD_DIR}"

  repo init --manifest-url "$MANIFEST_URL" --manifest-branch "$AOSP_BRANCH" --depth 1 || true
}

aosp_repo_modifications() {
  log_header ${FUNCNAME}
  cd "${BUILD_DIR}"

  # make modifications to default AOSP
  if ! grep -q "RattlesnakeOS" .repo/manifest.xml; then
    # really ugly awk script to add additional repos to manifest
    awk -i inplace \
      -v ANDROID_VERSION="$ANDROID_VERSION" \
      -v FDROID_CLIENT_VERSION="$FDROID_CLIENT_VERSION" \
      -v FDROID_PRIV_EXT_VERSION="$FDROID_PRIV_EXT_VERSION" \
      '1;/<repo-hooks in-project=/{
      print "  ";
      print "  <remote name=\"github\" fetch=\"https://github.com/RattlesnakeOS/\" revision=\"" ANDROID_VERSION "\" />";
      print "  <remote name=\"fdroid\" fetch=\"https://gitlab.com/fdroid/\" />";
      print "  <remote name=\"prepare-vendor\" fetch=\"https://github.com/anestisb/\" revision=\"master\" />";
      print "  ";
      print "  <project path=\"external/chromium\" name=\"platform_external_chromium\" remote=\"github\" />";
      print "  <project path=\"packages/apps/Updater\" name=\"platform_packages_apps_Updater\" remote=\"github\" />";
      print "  <project path=\"packages/apps/F-Droid\" name=\"fdroidclient\" remote=\"fdroid\" revision=\"refs/tags/" FDROID_CLIENT_VERSION "\" />";
      print "  <project path=\"packages/apps/F-DroidPrivilegedExtension\" name=\"privileged-extension\" remote=\"fdroid\" revision=\"refs/tags/" FDROID_PRIV_EXT_VERSION "\" />";
      print "  <project path=\"vendor/android-prepare-vendor\" name=\"android-prepare-vendor\" remote=\"prepare-vendor\" />"}' .repo/manifest.xml
  
    # remove things from manifest
    sed -i '/chromium-webview/d' .repo/manifest.xml
    sed -i '/packages\/apps\/Browser2/d' .repo/manifest.xml
    sed -i '/packages\/apps\/Calendar/d' .repo/manifest.xml
    sed -i '/packages\/apps\/QuickSearchBox/d' .repo/manifest.xml
  else
    log "Skipping modification of .repo/manifest.xml as they have already been made"
  fi
}

aosp_repo_sync() {
  log_header ${FUNCNAME}
  cd "${BUILD_DIR}"

  # sync with retries
  for i in {1..10}; do
    repo sync -c --no-tags --no-clone-bundle --jobs 32 && break
  done
}

setup_vendor() {
  log_header ${FUNCNAME}

  # get vendor files (with timeout)
  timeout 30m "${BUILD_DIR}/vendor/android-prepare-vendor/execute-all.sh" --debugfs --yes --device "${DEVICE}" --buildID "${AOSP_BUILD}" --output "${BUILD_DIR}/vendor/android-prepare-vendor"
  aws s3 cp - "s3://${AWS_RELEASE_BUCKET}/${DEVICE}-vendor" --acl public-read <<< "${AOSP_BUILD}" || true

  # copy vendor files to build tree
  mkdir --parents "${BUILD_DIR}/vendor/google_devices" || true
  rm --recursive --force "${BUILD_DIR}/vendor/google_devices/$DEVICE" || true
  mv "${BUILD_DIR}/vendor/android-prepare-vendor/${DEVICE}/$(tr '[:upper:]' '[:lower:]' <<< "${AOSP_BUILD}")/vendor/google_devices/${DEVICE}" "${BUILD_DIR}/vendor/google_devices"

  # smaller devices need big brother vendor files
  if [ "$DEVICE" == 'sailfish' ]; then
    rm --recursive --force "${BUILD_DIR}/vendor/google_devices/marlin" || true
    mv "${BUILD_DIR}/vendor/android-prepare-vendor/sailfish/$(tr '[:upper:]' '[:lower:]' <<< "${AOSP_BUILD}")/vendor/google_devices/marlin" "${BUILD_DIR}/vendor/google_devices"
  fi
  if [ "$DEVICE" == 'walleye' ]; then
    rm --recursive --force "${BUILD_DIR}/vendor/google_devices/muskie" || true
    mv "${BUILD_DIR}/vendor/android-prepare-vendor/walleye/$(tr '[:upper:]' '[:lower:]' <<< "${AOSP_BUILD}")/vendor/google_devices/muskie" "${BUILD_DIR}/vendor/google_devices"
  fi
}

aws_import_keys() {
  log_header ${FUNCNAME}

  if [ "$(aws s3 ls "s3://${AWS_KEYS_BUCKET}/${DEVICE}" | wc -l)" == '0' ]; then
    aws_gen_keys
  else
    log "Keys already exist for ${DEVICE} - grabbing them from S3"
    mkdir -p "${BUILD_DIR}/keys"
    retry aws s3 sync "s3://${AWS_KEYS_BUCKET}" "${BUILD_DIR}/keys"
  fi
}

apply_patches() {
  log_header ${FUNCNAME}

  patch_custom
  patch_aosp_removals
  patch_add_apps
  patch_base_config
  patch_device_config
  patch_chromium_webview
  patch_updater
  patch_fdroid
  patch_priv_ext
  patch_launcher
}

patch_aosp_removals() {
  log_header ${FUNCNAME}
  cd "${BUILD_DIR}"

  # remove aosp webview
  rm -rf platform/external/chromium-webview
  sed -i '/webview \\/d' build/make/target/product/core_minimal.mk

  # remove Browser2
  sed -i '/Browser2/d' build/make/target/product/core.mk

  # remove Calendar
  sed -i '/Calendar \\/d' build/make/target/product/core.mk

  # remove QuickSearchBox
  sed -i '/QuickSearchBox/d' build/make/target/product/core.mk
}

# TODO: most of this is fragile and unforgiving
patch_custom() {
  log_header ${FUNCNAME}
  
  # allow custom patches and shell scripts to be applied
  patches_dir="$HOME/patches"
  if [ -z "$REPO_PATCHES" ]; then
    log "No custom patches requested"
  else
    cd $BUILD_DIR
    log "Cloning custom patches $REPO_PATCHES to ${patches_dir}"
    retry git clone $REPO_PATCHES ${patches_dir}
    while read patch; do
      log "Applying patch $patch"
      case "$patch" in
          *.patch) patch -p1 < ${patches_dir}/$patch ;;
          *.sh)    . ${patches_dir}/$patch ;;
          *)       log "unknown patch type for ${patch}. skipping" ;;
      esac
    done < ${patches_dir}/manifest
  fi
  
  # allow prebuilt applications to be added to build tree
  prebuilt_dir="$BUILD_DIR/packages/apps/Custom"
  if [ -z "$REPO_PREBUILTS" ]; then
    log "No custom prebuilts requested"
  else
    log "Putting custom prebuilts from $REPO_PREBUILTS in build tree location ${prebuilt_dir}"
    retry git clone $REPO_PREBUILTS ${prebuilt_dir}
    while read package_name; do
      log "Adding custom PRODUCT_PACKAGES += ${package_name} to ${BUILD_DIR}/build/make/target/product/core.mk"
      sed -i "\$aPRODUCT_PACKAGES += ${package_name}" ${BUILD_DIR}/build/make/target/product/core.mk
    done < ${prebuilt_dir}/manifest
  fi

  # allow custom hosts file
  hosts_file_location="$BUILD_DIR/system/core/rootdir/etc/hosts"
  if [ -z "$HOSTS_FILE" ]; then
    log "No custom hosts file requested"
  else
    log "Replacing hosts file with $HOSTS_FILE"
    retry wget -O $hosts_file_location "$HOSTS_FILE"
  fi
}

patch_base_config() {
  log_header ${FUNCNAME}

  # enable swipe up gesture functionality as option
  sed -i 's@<bool name="config_swipe_up_gesture_setting_available">false</bool>@<bool name="config_swipe_up_gesture_setting_available">true</bool>@' ${BUILD_DIR}/frameworks/base/core/res/res/values/config.xml
}

patch_device_config() {
  log_header ${FUNCNAME}

  # set proper model names
  sed -i 's@PRODUCT_MODEL := AOSP on msm8996@PRODUCT_MODEL := Pixel XL@' ${BUILD_DIR}/device/google/marlin/aosp_marlin.mk
  sed -i 's@PRODUCT_MANUFACTURER := google@PRODUCT_MANUFACTURER := Google@' ${BUILD_DIR}/device/google/marlin/aosp_marlin.mk
  sed -i 's@PRODUCT_MODEL := AOSP on msm8996@PRODUCT_MODEL := Pixel@' ${BUILD_DIR}/device/google/marlin/aosp_sailfish.mk
  sed -i 's@PRODUCT_MANUFACTURER := google@PRODUCT_MANUFACTURER := Google@' ${BUILD_DIR}/device/google/marlin/aosp_sailfish.mk
  sed -i 's@PRODUCT_MODEL := AOSP on taimen@PRODUCT_MODEL := Pixel 2 XL@' ${BUILD_DIR}/device/google/taimen/aosp_taimen.mk
  sed -i 's@PRODUCT_MODEL := AOSP on walleye@PRODUCT_MODEL := Pixel 2@' ${BUILD_DIR}/device/google/muskie/aosp_walleye.mk
}

patch_chromium_webview() {
  log_header ${FUNCNAME}

  cat <<EOF > ${BUILD_DIR}/frameworks/base/core/res/res/xml/config_webview_packages.xml
<?xml version="1.0" encoding="utf-8"?>
<webviewproviders>
    <webviewprovider description="Chromium" packageName="org.chromium.chrome" availableByDefault="true">
    </webviewprovider>
</webviewproviders>
EOF
}

patch_fdroid() {
  log_header ${FUNCNAME}

  echo "sdk.dir=${HOME}/sdk" > ${BUILD_DIR}/packages/apps/F-Droid/local.properties
  echo "sdk.dir=${HOME}/sdk" > ${BUILD_DIR}/packages/apps/F-Droid/app/local.properties
  sed -i 's/gradle assembleRelease/..\/gradlew assembleRelease/' ${BUILD_DIR}/packages/apps/F-Droid/Android.mk
  sed -i 's@fdroid_apk   := build/outputs/apk/$(fdroid_dir)-release-unsigned.apk@fdroid_apk   := build/outputs/apk/full/release/app-full-release-unsigned.apk@'  ${BUILD_DIR}/packages/apps/F-Droid/Android.mk
}

patch_add_apps() {
  log_header ${FUNCNAME}

  sed -i "\$aPRODUCT_PACKAGES += Updater" ${BUILD_DIR}/build/make/target/product/core.mk
  sed -i "\$aPRODUCT_PACKAGES += F-DroidPrivilegedExtension" ${BUILD_DIR}/build/make/target/product/core.mk
  sed -i "\$aPRODUCT_PACKAGES += F-Droid" ${BUILD_DIR}/build/make/target/product/core.mk
  sed -i "\$aPRODUCT_PACKAGES += chromium" ${BUILD_DIR}/build/make/target/product/core.mk
}

patch_updater() {
  log_header ${FUNCNAME}

  cd "$BUILD_DIR"/packages/apps/Updater/res/values
  sed --in-place --expression "s@s3bucket@${RELEASE_URL}/@g" config.xml
}

fdpe_hash() {
  keytool -list -printcert -file "$1" | grep 'SHA256:' | tr --delete ':' | cut --delimiter ' ' --fields 3
}

patch_priv_ext() {
  log_header ${FUNCNAME}

  unofficial_releasekey_hash=$(fdpe_hash "${BUILD_DIR}/keys/${DEVICE}/releasekey.x509.pem")
  unofficial_platform_hash=$(fdpe_hash "${BUILD_DIR}/keys/${DEVICE}/platform.x509.pem")
  sed -i 's/'${OFFICIAL_FDROID_KEY}'")/'${unofficial_releasekey_hash}'"),\n            new Pair<>("org.fdroid.fdroid", "'${unofficial_platform_hash}'")/' \
      "${BUILD_DIR}/packages/apps/F-DroidPrivilegedExtension/app/src/main/java/org/fdroid/fdroid/privileged/ClientWhitelist.java"
}

patch_launcher() {
  log_header ${FUNCNAME}

  # disable QuickSearchBox widget on home screen
  sed -i.original "s/QSB_ON_FIRST_SCREEN = true;/QSB_ON_FIRST_SCREEN = false;/" "${BUILD_DIR}/packages/apps/Launcher3/src/com/android/launcher3/config/BaseFlags.java"
  # fix compile error with uninitialized variable
  sed -i.original "s/boolean createEmptyRowOnFirstScreen;/boolean createEmptyRowOnFirstScreen = false;/" "${BUILD_DIR}/packages/apps/Launcher3/src/com/android/launcher3/provider/ImportDataTask.java"
}

rebuild_marlin_kernel() {
  log_header ${FUNCNAME}

  # checkout kernel source on proper commit
  mkdir -p "${MARLIN_KERNEL_SOURCE_DIR}"
  retry git clone "${KERNEL_SOURCE_URL}" "${MARLIN_KERNEL_SOURCE_DIR}"
  # TODO: make this a bit more robust
  kernel_commit_id=$(lz4cat "${BUILD_DIR}/device/google/marlin-kernel/Image.lz4-dtb" | grep -a 'Linux version' | cut -d ' ' -f3 | cut -d'-' -f2 | sed 's/^g//g')
  cd "${MARLIN_KERNEL_SOURCE_DIR}"
  log "Checking out kernel commit ${kernel_commit_id}"
  git checkout ${kernel_commit_id}

  # run in another shell to avoid it mucking with environment variables for normal AOSP build
  bash -c "\
    set -e;
    cd ${BUILD_DIR};
    . build/envsetup.sh;
    make -j$(nproc --all) dtc mkdtimg;
    export PATH=${BUILD_DIR}/out/host/linux-x86/bin:${PATH};
    ln --verbose --symbolic ${BUILD_DIR}/keys/${DEVICE}/verity_user.der.x509 ${MARLIN_KERNEL_SOURCE_DIR}/verity_user.der.x509;
    cd ${MARLIN_KERNEL_SOURCE_DIR};
    make -j$(nproc --all) ARCH=arm64 marlin_defconfig;
    make -j$(nproc --all) ARCH=arm64 CONFIG_COMPAT_VDSO=n CROSS_COMPILE=${BUILD_DIR}/prebuilts/gcc/linux-x86/aarch64/aarch64-linux-android-4.9/bin/aarch64-linux-android-;
    cp -f arch/arm64/boot/Image.lz4-dtb ${BUILD_DIR}/device/google/marlin-kernel/;
    rm -rf ${BUILD_DIR}/out/build_*;
  "
}

build_aosp() {
  log_header ${FUNCNAME}

  cd "$BUILD_DIR"

  ############################
  # from original setup.sh script
  ############################
  source build/envsetup.sh
  export LANG=C
  export _JAVA_OPTIONS=-XX:-UsePerfData
  export BUILD_NUMBER=$(cat out/build_number.txt 2>/dev/null || date --utc +%Y.%m.%d.%H)
  log "BUILD_NUMBER=$BUILD_NUMBER"
  export DISPLAY_BUILD_NUMBER=true
  chrt -b -p 0 $$

  choosecombo $BUILD_TARGET
  log "Running target-files-package"
  retry make -j $(nproc) target-files-package
  log "Running brillo_update_payload"
  retry make -j $(nproc) brillo_update_payload
}

get_radio_image() {
  grep -Po "require version-$1=\K.+" vendor/$2/vendor-board-info.txt | tr '[:upper:]' '[:lower:]'
}

release() {
  log_header ${FUNCNAME}

  cd "$BUILD_DIR"

  ############################
  # from original setup.sh script
  ############################
  source build/envsetup.sh
  export LANG=C
  export _JAVA_OPTIONS=-XX:-UsePerfData
  export BUILD_NUMBER=$(cat out/build_number.txt 2>/dev/null || date --utc +%Y.%m.%d.%H)
  log "BUILD_NUMBER=$BUILD_NUMBER"
  export DISPLAY_BUILD_NUMBER=true
  chrt -b -p 0 $$

  ############################
  # from original release.sh script
  ############################
  KEY_DIR=keys/$1
  OUT=out/release-$1-${BUILD_NUMBER}
  source device/common/clear-factory-images-variables.sh

  DEVICE=$1
  BOOTLOADER=$(get_radio_image bootloader google_devices/${DEVICE})
  RADIO=$(get_radio_image baseband google_devices/${DEVICE})
  PREFIX=aosp_
  BUILD=$BUILD_NUMBER
  VERSION=$(grep -Po "export BUILD_ID=\K.+" build/core/build_id.mk | tr '[:upper:]' '[:lower:]')
  PRODUCT=${DEVICE}
  TARGET_FILES=$DEVICE-target_files-$BUILD.zip

  # make sure output directory exists
  mkdir -p $OUT

  # depending on device need verity key or avb key
  if [[ $DEVICE != taimen && $DEVICE != walleye ]]; then
    VERITY_SWITCHES=(--replace_verity_public_key "$KEY_DIR/verity_key.pub" --replace_verity_private_key "$KEY_DIR/verity"
                      --replace_verity_keyid "$KEY_DIR/verity.x509.pem")
  else
    VERITY_SWITCHES=(--avb_vbmeta_key "$KEY_DIR/avb.pem" --avb_vbmeta_algorithm SHA256_RSA2048)
  fi
  
  log "Running sign_target_files_apks"
  build/tools/releasetools/sign_target_files_apks -o -d "$KEY_DIR" "${VERITY_SWITCHES[@]}" \
    out/target/product/$DEVICE/obj/PACKAGING/target_files_intermediates/$PREFIX$DEVICE-target_files-$BUILD_NUMBER.zip \
    $OUT/$TARGET_FILES
  
  log "Running ota_from_target_files"
  build/tools/releasetools/ota_from_target_files --block -k "$KEY_DIR/releasekey" "${EXTRA_OTA[@]}" $OUT/$TARGET_FILES \
      $OUT/$DEVICE-ota_update-$BUILD.zip
  
  log "Running img_from_target_files"
  sed -i 's/zipfile\.ZIP_DEFLATED/zipfile\.ZIP_STORED/' build/tools/releasetools/img_from_target_files.py
  build/tools/releasetools/img_from_target_files $OUT/$TARGET_FILES $OUT/$DEVICE-img-$BUILD.zip
  
  log "Running generate-factory-images"
  cd $OUT
  sed -i 's/zip -r/tar cvf/' ../../device/common/generate-factory-images-common.sh
  sed -i 's/factory\.zip/factory\.tar/' ../../device/common/generate-factory-images-common.sh
  sed -i '/^mv / d' ../../device/common/generate-factory-images-common.sh
  source ../../device/common/generate-factory-images-common.sh
  mv $DEVICE-$VERSION-factory.tar $DEVICE-factory-$BUILD_NUMBER.tar
  rm -f $DEVICE-factory-$BUILD_NUMBER.tar.xz

  log "Running compress of factory image with pxz"
  time pxz -v -T0 -9 -z $DEVICE-factory-$BUILD_NUMBER.tar
}

# TODO: cleanup this function
aws_upload() {
  log_header ${FUNCNAME}

  cd "${BUILD_DIR}/out"
  build_date="$(< build_number.txt)"
  build_timestamp="$(unzip -p "release-${DEVICE}-${build_date}/${DEVICE}-ota_update-${build_date}.zip" META-INF/com/android/metadata | grep 'post-timestamp' | cut --delimiter "=" --fields 2)"

  # copy ota file to s3, update file metadata used by updater app, and remove old ota files
  read -r old_metadata <<< "$(wget -O - "${RELEASE_URL}/${RELEASE_CHANNEL}")"
  old_date="$(cut -d ' ' -f 1 <<< "${old_metadata}")"
  (
    aws s3 cp "${BUILD_DIR}/out/release-${DEVICE}-${build_date}/${DEVICE}-ota_update-${build_date}.zip" "s3://${AWS_RELEASE_BUCKET}" --acl public-read &&
    echo "${build_date} ${build_timestamp} ${AOSP_BUILD}" | aws s3 cp - "s3://${AWS_RELEASE_BUCKET}/${RELEASE_CHANNEL}" --acl public-read &&
    echo "${BUILD_TIMESTAMP}" | aws s3 cp - "s3://${AWS_RELEASE_BUCKET}/${RELEASE_CHANNEL}-true-timestamp" --acl public-read
  ) && ( aws s3 rm "s3://${AWS_RELEASE_BUCKET}/${DEVICE}-ota_update-${old_date}.zip" || true )

  # upload factory image
  retry aws s3 cp "${BUILD_DIR}/out/release-${DEVICE}-${build_date}/${DEVICE}-factory-${build_date}.tar.xz" "s3://${AWS_RELEASE_BUCKET}/${DEVICE}-factory-latest.tar.xz"

  # cleanup old target files if some exist
  if [ "$(aws s3 ls "s3://${AWS_RELEASE_BUCKET}/${DEVICE}-target" | wc -l)" != '0' ]; then
    cleanup_target_files
  fi

  # copy new target file to s3
  retry aws s3 cp "${BUILD_DIR}/out/release-${DEVICE}-${build_date}/${DEVICE}-target_files-${build_date}.zip" "s3://${AWS_RELEASE_BUCKET}/${DEVICE}-target/${DEVICE}-target-files-${build_date}.zip"
}

cleanup_target_files() {
  log_header ${FUNCNAME}

  aws s3 sync "s3://${AWS_RELEASE_BUCKET}/${DEVICE}-target" "${BUILD_DIR}/${DEVICE}-target"
  cd "${BUILD_DIR}/${DEVICE}-target"
  for target_file in ${DEVICE}-target-files-*.zip ; do
    old_date=$(echo "$target_file" | cut --delimiter "-" --fields 4 | cut --delimiter "." --fields 5 --complement)
    aws s3 rm "s3://${AWS_RELEASE_BUCKET}/${DEVICE}-target/${DEVICE}-target-files-${old_date}.zip" || true
  done
}

checkpoint_versions() {
  log_header ${FUNCNAME}

  # checkpoint stack version
  echo "${STACK_VERSION}" | aws s3 cp - "s3://${AWS_RELEASE_BUCKET}/rattlesnakeos-stack/revision"

  # checkpoint f-droid
  echo "${FDROID_PRIV_EXT_VERSION}" | aws s3 cp - "s3://${AWS_RELEASE_BUCKET}/fdroid-priv/revision"
  echo "${FDROID_CLIENT_VERSION}" | aws s3 cp - "s3://${AWS_RELEASE_BUCKET}/fdroid/revision"
}

aws_notify_simple() {
  log_header ${FUNCNAME}

  aws sns publish --region ${REGION} --topic-arn "$AWS_SNS_ARN" --message "$1"
}

aws_notify() {
  log_header ${FUNCNAME}

  LOGOUTPUT=
  if [ ! -z "$2" ]; then
    LOGOUTPUT=$(tail -c 20000 /var/log/cloud-init-output.log)
  fi
  ELAPSED="$(($SECONDS / 3600))hrs $((($SECONDS / 60) % 60))min $(($SECONDS % 60))sec"
  aws sns publish --region ${REGION} --topic-arn "$AWS_SNS_ARN" \
    --message="$(printf "$1\n  Device: %s\n  Stack Name: %s\n  Stack Version: %s %s\n  Stack Region: %s\n  Release Channel: %s\n  Instance Type: %s\n  Instance Region: %s\n  Instance IP: %s\n  Build Date: %s\n  Elapsed Time: %s\n  AOSP Build: %s\n  AOSP Branch: %s\n  Chromium Version: %s\n  F-Droid Version: %s\n  F-Droid Priv Extension Version: %s\n%s" \
      "${DEVICE}" "${STACK_NAME}" "${STACK_VERSION}" "${STACK_UPDATE_MESSAGE}" "${REGION}" "${RELEASE_CHANNEL}" "${INSTANCE_TYPE}" "${INSTANCE_REGION}" "${INSTANCE_IP}" "${BUILD_DATE}" "${ELAPSED}" "${AOSP_BUILD}" "${AOSP_BRANCH}" "${LATEST_CHROMIUM}" "${FDROID_CLIENT_VERSION}" "${FDROID_PRIV_EXT_VERSION}" "${LOGOUTPUT}")" || true
}

aws_logging() {
  log_header ${FUNCNAME}

  df -h
  du -chs "${BUILD_DIR}" || true
  uptime
  aws s3 cp /var/log/cloud-init-output.log "s3://${AWS_LOGS_BUCKET}/${DEVICE}/$(date +%s)"
}

aws_gen_keys() {
  log_header ${FUNCNAME}

  gen_keys
  retry aws s3 sync "${BUILD_DIR}/keys" "s3://${AWS_KEYS_BUCKET}"
}

gen_keys() {
  log_header ${FUNCNAME}

  mkdir --parents "${BUILD_DIR}/keys/${DEVICE}"
  cd "${BUILD_DIR}/keys/${DEVICE}"
  for key in {releasekey,platform,shared,media,verity} ; do
    # make_key exits with unsuccessful code 1 instead of 0, need ! to negate
    ! "${BUILD_DIR}/development/tools/make_key" "$key" "$CERTIFICATE_SUBJECT"
  done

  if [ "${DEVICE}" == "marlin" ] || [ "${DEVICE}" == "sailfish" ]; then
    gen_verity_key "${DEVICE}"
  fi

  if [ "${DEVICE}" == "walleye" ] || [ "${DEVICE}" == "taimen" ]; then
    gen_avb_key "${DEVICE}"
  fi
}

gen_avb_key() {
  log_header ${FUNCNAME}

  cd "$BUILD_DIR"
  openssl genrsa -out "${BUILD_DIR}/keys/$1/avb.pem" 2048
  ${BUILD_DIR}/external/avb/avbtool extract_public_key --key "${BUILD_DIR}/keys/$1/avb.pem" --output "${BUILD_DIR}/keys/$1/avb_pkmd.bin"
}

gen_verity_key() {
  log_header ${FUNCNAME}
  cd "$BUILD_DIR"

  make -j 20 generate_verity_key
  "${BUILD_DIR}/out/host/linux-x86/bin/generate_verity_key" -convert "${BUILD_DIR}/keys/$1/verity.x509.pem" "${BUILD_DIR}/keys/$1/verity_key"
  make clobber
  openssl x509 -outform der -in "${BUILD_DIR}/keys/$1/verity.x509.pem" -out "${BUILD_DIR}/keys/$1/verity_user.der.x509"
}

cleanup() {
  log_header ${FUNCNAME}

  rv=$?
  aws_logging
  if [ $rv -ne 0 ]; then
    aws_notify "RattlesnakeOS Build FAILED" 1
  fi
  if [ "${PREVENT_SHUTDOWN}" = true ]; then
    log "Skipping shutdown"
  else
    sudo shutdown -h now
  fi
}

log_header() {
  echo "=================================="
  echo "$(date "+%Y-%m-%d %H:%M:%S"): Running $1"
  echo "=================================="
}

log() {
  echo "$(date "+%Y-%m-%d %H:%M:%S"): $1"
}

retry() {
  set +e
  local max_attempts=${ATTEMPTS-3}
  local timeout=${TIMEOUT-1}
  local attempt=0
  local exitCode=0

  while [[ $attempt < $max_attempts ]]
  do
    "$@"
    exitCode=$?

    if [[ $exitCode == 0 ]]
    then
      break
    fi

    log "Failure! Retrying ($@) in $timeout.."
    sleep $timeout
    attempt=$(( attempt + 1 ))
    timeout=$(( timeout * 2 ))
  done

  if [[ $exitCode != 0 ]]
  then
    log "Failed too many times! ($@)"
  fi

  set -e

  return $exitCode
}

trap cleanup 0

set -e

full_run
`
