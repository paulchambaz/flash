# Run the CLI
run *ARGS:
  #!/usr/bin/env bash
  set -euo pipefail
  args=()
  for arg in {{ ARGS }}; do
    [[ -e "$arg" ]] && args+=("$(realpath "$arg")") || args+=("$arg")
  done
  cd cli && go run . "${args[@]}"

# Build a debug APK
build-android:
  cd android && ./gradlew assembleDebug ${AAPT2:+-Pandroid.aapt2FromMavenOverride=$AAPT2}

# Install debug APK on connected device
install-android:
  cd android && ./gradlew installDebug ${AAPT2:+-Pandroid.aapt2FromMavenOverride=$AAPT2}

# Install and launch on connected device
run-android: install-android
  adb -s $(adb-device) shell am start -n xyz.chambaz.flash/.MainActivity

# Build a release APK
release-android:
  cd android && ./gradlew assembleRelease ${AAPT2:+-Pandroid.aapt2FromMavenOverride=$AAPT2}

# Uninstall from connected device
uninstall-android:
  adb -s $(adb-device) uninstall xyz.chambaz.flash

# Delete Android build outputs
clean-android:
  cd android && ./gradlew clean

# Stream device logs filtered to flash
log-android:
  adb -s $(adb-device) logcat | grep xyz.chambaz.flash

# List connected ADB devices
devices:
  adb devices

# Pair wirelessly via QR — phone: Wireless Debugging → Pair with QR code
pair:
  adb-pair

# Connect to saved device
connect:
  adb-device

# Create Android signing key
signkey-android:
  keytool -genkey -v -keystore ~/.android/flash-release.jks -alias flash \
    -keyalg EC -keysize 256 -validity 10000 \
    -dname "CN=Paul Chambaz, O=Flash, C=FR"
