# When changing this file, you need to bump the following
# .gitlab-ci/image-tags.yml tags:
# ALPINE_X86_64_LAVA_TRIGGER_TAG

import re
from dataclasses import dataclass
from datetime import datetime, timedelta
from enum import Enum, auto
from os import getenv
from typing import Optional, Pattern, Union

from lava.utils.gitlab_section import GitlabSection


class LogSectionType(Enum):
    UNKNOWN = auto()
    LAVA_SUBMIT = auto()
    LAVA_QUEUE = auto()
    LAVA_DEPLOY = auto()
    LAVA_BOOT = auto()
    TEST_SUITE = auto()
    TEST_CASE = auto()
    LAVA_POST_PROCESSING = auto()


# How long to wait whilst we try to submit a job; make it fairly short,
# since the job will be retried.
LAVA_SUBMIT_TIMEOUT = int(getenv("LAVA_SUBMIT_TIMEOUT", 5))

# How long should we wait for a device to become available?
# For post-merge jobs, this should be ~infinite, but we can fail more
# aggressively for pre-merge.
LAVA_QUEUE_TIMEOUT = int(getenv("LAVA_QUEUE_TIMEOUT", 60))

# How long should we wait for a device to be deployed?
# The deploy involves downloading and decompressing the kernel, modules, dtb and the overlays.
# We should retry, to overcome network issues.
LAVA_DEPLOY_TIMEOUT = int(getenv("LAVA_DEPLOY_TIMEOUT", 5))

# Empirically, successful device deploy+boot in LAVA time takes less than 3 minutes.
# LAVA itself is configured to attempt `failure_retry` times (NUMBER_OF_ATTEMPTS_LAVA_BOOT) to boot
# the device.
# It is better to retry the boot than cancel the job and re-submit to avoid
# the enqueue delay.
LAVA_BOOT_TIMEOUT = int(getenv("LAVA_BOOT_TIMEOUT", 5))

# Estimated overhead in minutes for a job from GitLab to reach the test phase,
# including LAVA scheduling and boot duration
LAVA_TEST_OVERHEAD_MIN = int(getenv("LAVA_TEST_OVERHEAD_MIN", 5))

# CI_JOB_TIMEOUT in full minutes, no reason to use seconds here
CI_JOB_TIMEOUT_MIN = int(getenv("CI_JOB_TIMEOUT")) // 60
# Sanity check: we need more job time than the LAVA estimated overhead
assert CI_JOB_TIMEOUT_MIN > LAVA_TEST_OVERHEAD_MIN, (
    f"CI_JOB_TIMEOUT in full minutes ({CI_JOB_TIMEOUT_MIN}) must be greater than LAVA_TEST_OVERHEAD ({LAVA_TEST_OVERHEAD_MIN})"
)

# Test suite phase is where initialization occurs on both the DUT and the Docker container.
# The device will be listening to the SSH session until the end of the job.
LAVA_TEST_SUITE_TIMEOUT = CI_JOB_TIMEOUT_MIN - LAVA_TEST_OVERHEAD_MIN

# Test cases may take a long time, this script has no right to interrupt
# them. But if the test case takes almost 1h, it will never succeed due to
# Gitlab job timeout.
LAVA_TEST_CASE_TIMEOUT = CI_JOB_TIMEOUT_MIN - LAVA_TEST_OVERHEAD_MIN

# LAVA post processing may refer to a test suite teardown, or the
# adjustments to start the next test_case
LAVA_POST_PROCESSING_TIMEOUT = int(getenv("LAVA_POST_PROCESSING_TIMEOUT", 5))

FALLBACK_GITLAB_SECTION_TIMEOUT = timedelta(minutes=10)
DEFAULT_GITLAB_SECTION_TIMEOUTS = {
    LogSectionType.LAVA_SUBMIT: timedelta(minutes=LAVA_SUBMIT_TIMEOUT),
    LogSectionType.LAVA_QUEUE: timedelta(minutes=LAVA_QUEUE_TIMEOUT),
    LogSectionType.LAVA_DEPLOY: timedelta(minutes=LAVA_DEPLOY_TIMEOUT),
    LogSectionType.LAVA_BOOT: timedelta(minutes=LAVA_BOOT_TIMEOUT),
    LogSectionType.TEST_SUITE: timedelta(minutes=LAVA_TEST_SUITE_TIMEOUT),
    LogSectionType.TEST_CASE: timedelta(minutes=LAVA_TEST_CASE_TIMEOUT),
    LogSectionType.LAVA_POST_PROCESSING: timedelta(
        minutes=LAVA_POST_PROCESSING_TIMEOUT
    ),
}


@dataclass(frozen=True)
class LogSection:
    regex: Union[Pattern, str]
    levels: tuple[str]
    section_id: str
    section_header: str
    section_type: LogSectionType
    collapsed: bool = False

    def from_log_line_to_section(
        self, lava_log_line: dict[str, str], main_test_case: Optional[str],
        timestamp_relative_to: Optional[datetime]
    ) -> Optional[GitlabSection]:
        if lava_log_line["lvl"] not in self.levels:
            return

        if match := re.search(self.regex, lava_log_line["msg"]):
            section_id = self.section_id.format(*match.groups())
            section_header = self.section_header.format(*match.groups())
            is_main_test_case = section_id == main_test_case
            return GitlabSection(
                id=section_id,
                header=section_header,
                type=self.section_type,
                start_collapsed=self.collapsed,
                suppress_start=is_main_test_case,
                suppress_end=is_main_test_case,
                timestamp_relative_to=timestamp_relative_to,
            )


LOG_SECTIONS = (
    LogSection(
        regex=re.compile(r"start: 2 (\S+) \(timeout ([^)]+)\).*"),
        levels=("info"),
        section_id="{}",
        section_header="Booting via {}",
        section_type=LogSectionType.LAVA_BOOT,
        collapsed=True,
    ),
    LogSection(
        regex=re.compile(r"<?STARTTC>? ([^>]*)"),
        levels=("target", "debug"),
        section_id="{}",
        section_header="test_case {}",
        section_type=LogSectionType.TEST_CASE,
        collapsed=True,
    ),
    LogSection(
        regex=re.compile(r"<?STARTRUN>? ([^>]*ssh.*server.*)"),
        levels=("debug"),
        section_id="{}",
        section_header="Setting up hardware device for remote control",
        section_type=LogSectionType.TEST_SUITE,
        collapsed=True,
    ),
    LogSection(
        regex=re.compile(r"ENDTC>? ([^>]+)"),
        levels=("target", "debug"),
        section_id="post-{}",
        section_header="Post test_case {}",
        section_type=LogSectionType.LAVA_POST_PROCESSING,
        collapsed=True,
    ),
)
