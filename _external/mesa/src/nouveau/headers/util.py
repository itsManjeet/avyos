import subprocess


def _render_template(template, environment):
    try:
        return template.render(**environment)
    except Exception:
        # In the event there's an error, this imports some helpers from mako
        # to print a useful stack trace and prints it, then exits with
        # status 1, if python is run with debug; otherwise it just raises
        # the exception
        import sys
        from mako import exceptions

        print(exceptions.text_error_template().render(), file=sys.stderr)
        sys.exit(1)


def write_template(out_file, template, environment):
    with open(out_file, "w", encoding="utf-8") as f:
        f.write(_render_template(template, environment))


def write_template_rs(out_file, template, environment):
    contents = _render_template(template, environment)

    try:
        contents = subprocess.check_output(
            ["rustfmt"],
            input=contents,
            text=True,
        )
    except (subprocess.CalledProcessError, FileNotFoundError, PermissionError):
        pass

    with open(out_file, "w", encoding="utf-8") as f:
        f.write(contents)
