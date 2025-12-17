# Copyright (C) 2025 fontconfig Authors
# SPDX-License-Identifier: HPND

from fctest import FcTest, FcExternalTestFont, FcTestFont
from pathlib import Path
from tempfile import NamedTemporaryFile
import pytest


@pytest.fixture
def fctest():
    return FcTest()


@pytest.fixture
def fcfont():
    return FcTestFont()


@pytest.mark.parametrize("font_file", FcExternalTestFont().fonts)
def test_genconf(fctest, fcfont, font_file):
    font_path = Path(font_file)

    if not font_path.exists():
        pytest.skip(f"Font file not found: {font_file}")

    extraconffile = NamedTemporaryFile(prefix='fontconfig.',
                                       suffix='.extra.conf',
                                       mode='w',
                                       delete_on_close=False)
    fctest._extra.append(f'<include ignore_missing="yes">{fctest.convert_path(extraconffile.name)}</include>')
    fctest.setup()
    fctest.install_font(fcfont.fonts, '.')
    fctest.install_font(font_file, '.')

    f = g = fmt = None
    for ret, stdout, stderr in fctest.run_query(['-f', '%{family}:%{genericfamily}\n', font_file]):
        assert ret == 0, stderr
        for l in stdout.strip().splitlines():
            f, g = l.split(':')
            assert f, stdout
            if not g or g == '0':
                # fallback if missing
                g = 'sans-serif'
                fmt = ':family=sans-serif'
            else:
                fmt = f':genericfamily={g}'

    for ret, stdout, stderr in fctest.run_genconf(['-g', g, '-f', f, '-o', extraconffile.name, font_file]):
        assert ret == 0, stderr

    for ret, stdout, stderr in fctest.run_match(['-f', '%{file}\n', fmt]):
        assert ret == 0, stderr
        if Path(stdout.strip().splitlines()[0]).name != font_path.name:
            assert ret == 0, stderr
