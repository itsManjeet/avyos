import importlib
import os
import re
import subprocess
from itertools import chain
from pathlib import Path
from typing import Any, Iterable, Literal
from unittest import mock

import lava.utils.constants
import pytest
from lava.lava_job_submitter import LAVAJobSubmitter
from lava.utils.lava_job_definition import LAVAJobDefinition
from ruamel.yaml import YAML


def flatten(iterable: Iterable[Iterable[Any]]) -> list[Any]:
    return list(chain.from_iterable(iterable))


# mock shell file
@pytest.fixture(scope="session")
def shell_file(tmp_path_factory):
    def create_shell_file(content: str = "# test"):
        shell_file = tmp_path_factory.mktemp("data") / "shell_file.sh"
        shell_file.write_text(content)
        return shell_file

    return create_shell_file


# fn to load the data file from $CWD/data using pathlib
def load_data_file(filename):
    return Path(__file__).parent.parent / "data" / filename


def load_yaml_file(filename) -> dict:
    with open(load_data_file(filename)) as f:
        return YAML().load(f)


def job_submitter_factory(mode: Literal["UBOOT", "FASTBOOT"], shell_file):
    if mode == "UBOOT":
        boot_method = "u-boot"
        device_type = "my_uboot_device_type"
    elif mode == "FASTBOOT":
        boot_method = "fastboot"
        device_type = "my_fastboot_device_type"

    job_timeout_min = 10
    mesa_job_name = "dut test"
    pipeline_info = "my_pipeline_info"
    project_name = "test-project"
    visibility_group = "my_visibility_group"

    return LAVAJobSubmitter(
        boot_method=boot_method,
        device_type=device_type,
        farm="test_farm",
        dtb_filename="my_dtb_filename",
        first_stage_init=shell_file,
        env_file=shell_file,
        job_timeout_min=job_timeout_min,
        mesa_job_name=mesa_job_name,
        pipeline_info=pipeline_info,
        visibility_group=visibility_group,
        project_dir="/test_dir",
        project_name=project_name,
    )


@pytest.fixture
def clear_env_vars(autouse=True):
    with mock.patch.dict(os.environ) as environ:
        # Remove all LAVA-related environment variables to make the test more robust
        # and deterministic, once a envvar is capable of overriding the default value
        for key in environ:
            if any(kw in key for kw in ("LAVA_", "CI_", "JOB_", "RUNNER_", "DEVICE_")):
                del environ[key]
        # reload lava.utils.constants to update the JOB_PRIORITY value
        importlib.reload(lava.utils.constants)
        importlib.reload(lava.utils.lava_job_definition)
        yield


@pytest.fixture
def mock_collabora_farm(clear_env_vars, monkeypatch):
    # Mock a Collabora farm-like device runner tag to enable SSH execution
    monkeypatch.setenv("FARM", "collabora")


@pytest.mark.parametrize("force_uart", [True, False], ids=["SSH", "UART"])
@pytest.mark.parametrize("mode", ["UBOOT", "FASTBOOT"])
@mock.patch("lava.lava_job_submitter.setup_lava_proxy")
def test_generate_lava_job_definition_sanity(
    mock_lava_proxy,
    force_uart,
    mode,
    shell_file,
    mock_collabora_farm,
    monkeypatch,
    mock_proxy,
):
    monkeypatch.setattr(lava.utils.lava_job_definition, "FORCE_UART", force_uart)
    # Do not actually connect to the LAVA server
    mock_lava_proxy.return_value = mock_proxy

    init_script_content = f"echo test {mode}"
    job_submitter = job_submitter_factory(mode, shell_file(init_script_content))
    job_definition = LAVAJobDefinition(job_submitter).generate_lava_job_definition()

    # Load the YAML output and check that it contains the expected keys and values
    yaml = YAML()
    job_dict = yaml.load(job_definition)
    yaml.dump(job_dict, Path(f"/tmp/{mode}_force_uart={force_uart}_job_definition.yaml"))
    assert job_dict["device_type"] == job_submitter.device_type
    assert job_dict["visibility"]["group"] == [job_submitter.visibility_group]
    assert job_dict["timeouts"]["job"]["minutes"] == job_submitter.job_timeout_min
    assert job_dict["context"]["extra_nfsroot_args"]
    assert job_dict["timeouts"]["actions"]

    assert len(job_dict["actions"]) == 3 if mode == "UART" else 5

    last_test_action = job_dict["actions"][-1]["test"]
    # TODO: Remove hardcoded "mesa" test name, as this submitter is being used by other projects
    first_test_name = last_test_action["definitions"][0]["name"]
    is_running_ssh = "ssh" in first_test_name
    # if force_uart, is_ssh must be False. If is_ssh, force_uart must be False. Both can be False
    assert not (is_running_ssh and force_uart)
    assert last_test_action["failure_retry"] == 3 if is_running_ssh else 1

    run_steps = "".join(last_test_action["definitions"][0]["repository"]["run"]["steps"])
    # Check for project name in lava-test-case
    assert re.search(rf"lava.?\S*.test.case.*{job_submitter.project_name}", run_steps)

    action_names = flatten(j.keys() for j in job_dict["actions"])
    if is_running_ssh:
        assert action_names == (
            [
                "deploy",
                "boot",
                "test",  # DUT: SSH server
                "test",  # Docker: SSH client
            ]
            if mode == "UBOOT"
            else [
                "deploy",  # NFS
                "deploy",  # Image generation
                "deploy",  # Image deployment
                "boot",
                "test",  # DUT: SSH server
                "test",  # Docker: SSH client
            ]
        )
        test_action_server = job_dict["actions"][-2]["test"]
        # SSH server in the DUT
        assert test_action_server["namespace"] == "dut"
        # SSH client via docker
        assert last_test_action["namespace"] == "container"

        boot_action = next(a["boot"] for a in job_dict["actions"] if "boot" in a)
        assert boot_action["namespace"] == "dut"

        # SSH server bootstrapping
        assert "dropbear" in "".join(boot_action["auto_login"]["login_commands"])
        return

    # ---- Not SSH job
    assert action_names == (
        [
            "deploy",
            "boot",
            "test",
        ]
        if mode == "UBOOT"
        else [
            "deploy",  # NFS
            "deploy",  # Image generation
            "deploy",  # Image deployment
            "boot",
            "test",
        ]
    )
    assert init_script_content in run_steps


# use yaml files from tests/data/ to test the job definition generation
@pytest.mark.parametrize("force_uart", [False, True], ids=["SSH", "UART"])
@pytest.mark.parametrize("mode", ["UBOOT", "FASTBOOT"])
@mock.patch("lava.lava_job_submitter.setup_lava_proxy")
def test_lava_job_definition(
    mock_lava_proxy,
    mode,
    force_uart,
    shell_file,
    mock_collabora_farm,
    mock_proxy,
    monkeypatch,
):
    monkeypatch.setattr(lava.utils.lava_job_definition, "FORCE_UART", force_uart)
    # Do not actually connect to the LAVA server
    mock_lava_proxy.return_value = mock_proxy

    yaml = YAML()
    yaml.default_flow_style = False

    # Load the YAML output and check that it contains the expected keys and values
    expected_job_dict = load_yaml_file(f"{mode}_force_uart={force_uart}_job_definition.yaml")

    init_script_content = f"echo test {mode}"
    job_submitter = job_submitter_factory(mode, shell_file(init_script_content))
    job_definition = LAVAJobDefinition(job_submitter).generate_lava_job_definition()

    job_dict = yaml.load(job_definition)

    # Uncomment the following to update the expected YAML files
    # yaml.dump(job_dict, load_data_file(f"{mode}_force_uart={force_uart}_job_definition.yaml"))

    # Check that the generated job definition matches the expected one
    assert job_dict == expected_job_dict


@pytest.mark.parametrize(
    "directive",
    ["declare -x", "export"],
)
@pytest.mark.parametrize(
    "original_env_output",
    [
        # Test basic environment variables
        "FOO=bar\nBAZ=qux",
        # Test export statements
        "{directive} FOO=bar",
        # Test multiple exports
        "{directive} FOO=bar\n{directive} BAZ=qux\nNORM=val",
        # Test mixed content with export
        "{directive} FOO=bar\nBAZ=qux\n{directive} HELLO=world",
        # Test empty file
        "",
        # Test special characters that need shell quoting
        "FOO='bar baz'\nQUOTE=\"hello world\"",
        # Test variables with spaces and quotes
        "{directive} VAR='val spaces'\nQUOTES=\"test\"",
        # Test inline scripts with export
        "{directive} FOO=bar\nBAZ=qux\n{directive} HELLO=world",
        # Test single quote inside double quotes in variable
        "{directive} FOO='Revert \"commit's error\"'",
        # Test backticks in variable
        "{directive} FOO=`echo 'test'`",
    ],
    ids=[
        "basic_vars",
        "single_export",
        "multiple_exports",
        "mixed_exports",
        "empty_file",
        "special_chars",
        "spaces_and_quotes",
        "inline_scripts_with_export",
        "single_quote_in_var",
        "backticks",
    ]
)
def test_encode_job_env_vars(directive, original_env_output, shell_file, clear_env_vars):
    """Test the encode_job_env_vars function with various environment file contents."""
    import base64
    import shlex

    # Create environment file with test content
    original_env_output = original_env_output.format(directive=directive)
    env_file = shell_file(original_env_output)

    # Create job submitter with the environment file
    job_submitter = mock.MagicMock(spec=LAVAJobSubmitter, env_file=env_file)
    job_definition = LAVAJobDefinition(job_submitter)

    # Call the function under test
    result = job_definition.encode_job_env_vars()

    # Verify the result is a list with exactly one element
    assert isinstance(result, list)
    assert len(result) == 1

    # Extract the command from the result
    command = result[0]
    assert isinstance(command, str)

    # Extract the base64 encoded part
    start_marker = 'echo '
    end_marker = ' | base64 -d'

    start_idx = command.find(start_marker) + len(start_marker)
    end_idx = command.find(end_marker)
    redirect_idx = command.find(">")
    encoded_part = command[start_idx:end_idx]

    # Verify if the script is executed correctly
    env_script_process = subprocess.run(
        ["bash", "-c", command[:redirect_idx]], capture_output=True, text=True
    )

    if env_script_process.returncode != 0:
        pytest.fail(f"Failed to execute script: {env_script_process.stderr}")

    generated_env_output = env_script_process.stdout.strip()

    # The encoded part should be shell-quoted, so we need to parse it
    # Use shlex to unquote the encoded content
    unquoted_encoded = shlex.split(encoded_part)[0]

    # Decode the base64 content
    try:
        decoded_content = base64.b64decode(unquoted_encoded).decode()
    except Exception as e:
        pytest.fail(f"Failed to decode base64 content: {e}. Encoded part: {encoded_part}")

    # Verify the decoded content matches the original file content
    assert decoded_content == original_env_output == generated_env_output
