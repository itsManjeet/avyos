#!/usr/bin/env bash
# shellcheck disable=SC2086 # we want word splitting
# shellcheck disable=SC1091 # paths only become valid at runtime

. "${SCRIPTS_DIR}/setup-test-env.sh"

# deqp

$ADB shell mkdir -p /data/deqp
$ADB push /deqp-gles/modules/egl/deqp-egl-android /data/deqp
$ADB push /deqp-gles/mustpass/egl-main.txt.zst /data/deqp
$ADB push /deqp-gles/modules/gles2/deqp-gles2 /data/deqp
$ADB push /deqp-gles/mustpass/gles2-main.txt.zst /data/deqp
$ADB push /deqp-vk/external/vulkancts/modules/vulkan/* /data/deqp
$ADB push /deqp-vk/mustpass/vk-main.txt.zst /data/deqp
$ADB push /deqp-tools/* /data/deqp
$ADB push /deqp-runner/deqp-runner /data/deqp

$ADB push "$INSTALL/all-skips.txt" /data/deqp
$ADB push "$INSTALL/android-skips.txt" /data/deqp
$ADB push "$INSTALL/angle-skips.txt" /data/deqp
if [ -e "$INSTALL/$GPU_VERSION-flakes.txt" ]; then
  $ADB push "$INSTALL/$GPU_VERSION-flakes.txt" /data/deqp
fi
if [ -e "$INSTALL/$GPU_VERSION-fails.txt" ]; then
  $ADB push "$INSTALL/$GPU_VERSION-fails.txt" /data/deqp
fi
if [ -e "$INSTALL/$GPU_VERSION-skips.txt" ]; then
  $ADB push "$INSTALL/$GPU_VERSION-skips.txt" /data/deqp
fi
$ADB push "$INSTALL/deqp-$DEQP_SUITE.toml" /data/deqp

BASELINE=""
if [ -e "$INSTALL/$GPU_VERSION-fails.txt" ]; then
    BASELINE="--baseline /data/deqp/$GPU_VERSION-fails.txt"
fi

# Default to an empty known flakes file if it doesn't exist.
$ADB shell "touch /data/deqp/$GPU_VERSION-flakes.txt"

DEQP_SKIPS=""
if [ -e "$INSTALL/$GPU_VERSION-skips.txt" ]; then
    DEQP_SKIPS="$DEQP_SKIPS /data/deqp/$GPU_VERSION-skips.txt"
fi

if [ -n "${ANGLE_TAG:-}" ]; then
    DEQP_SKIPS="$DEQP_SKIPS /data/deqp/angle-skips.txt"
fi

AOSP_RESULTS=/data/deqp/results
uncollapsed_section_switch cuttlefish_test "cuttlefish: testing"

# Print the detailed version with the list of backports and local patches
{ set +x; } 2>/dev/null
for api in vk-main vk gl gles; do
  deqp_version_log=/deqp-$api/deqp-$api-version
  if [ -r "$deqp_version_log" ]; then
    cat "$deqp_version_log"
  fi
done
set -x

set +e
$ADB shell "mkdir ${AOSP_RESULTS}; cd ${AOSP_RESULTS}/..; \
  XDG_CACHE_HOME=/data/local/tmp \
  ./deqp-runner \
    suite \
    --suite /data/deqp/deqp-$DEQP_SUITE.toml \
    --output $AOSP_RESULTS \
    --skips /data/deqp/all-skips.txt $DEQP_SKIPS \
    --flakes /data/deqp/$GPU_VERSION-flakes.txt \
    --testlog-to-xml /data/deqp/testlog-to-xml \
    --shader-cache-dir /data/local/tmp \
    --fraction-start ${CI_NODE_INDEX:-1} \
    --fraction $(( CI_NODE_TOTAL * ${DEQP_FRACTION:-1})) \
    --jobs ${FDO_CI_CONCURRENT:-4} \
    $BASELINE \
    ${DEQP_RUNNER_MAX_FAILS:+--max-fails \"$DEQP_RUNNER_MAX_FAILS\"} \
    "

# shellcheck disable=SC2034 # EXIT_CODE is used by the script that sources this one
EXIT_CODE=$?
set -e
section_switch cuttlefish_results "cuttlefish: gathering the results"

$ADB pull "$AOSP_RESULTS/." "$RESULTS_DIR"

# Remove all but the first 50 individual XML files uploaded as artifacts, to
# save fd.o space when you break everything.
find $RESULTS_DIR -name \*.xml | \
    sort -n |
    sed -n '1,+49!p' | \
    xargs rm -f

# If any QPA XMLs are there, then include the XSL/CSS in our artifacts.
find $RESULTS_DIR -name \*.xml \
    -exec cp /deqp-tools/testlog.css /deqp-tools/testlog.xsl "$RESULTS_DIR/" ";" \
    -quit

$ADB shell "cd ${AOSP_RESULTS}/..; \
./deqp-runner junit \
   --testsuite dEQP \
   --results $AOSP_RESULTS/failures.csv \
   --output $AOSP_RESULTS/junit.xml \
   --limit 50 \
   --template \"See $ARTIFACTS_BASE_URL/results/{{testcase}}.xml\""

$ADB pull "$AOSP_RESULTS/junit.xml" "$RESULTS_DIR"

section_end cuttlefish_results
