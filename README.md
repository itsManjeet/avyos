# libdisplay-info

EDID and DisplayID library.

Goals:

- Provide a set of high-level, easy-to-use, opinionated functions as well as
  low-level functions to access detailed information.
- Simplicity and correctness over performance and resource usage.
- Well-tested and fuzzed.

Documentation is available on the [website].

## Using

The public API headers are categorised as either high-level or low-level API
as per the comments in the header files. Users of libdisplay-info should prefer
high-level API over low-level API when possible.

If high-level API lacks needed features, please propose additions to the
high-level API upstream before using low-level API to get what you need.
If the additions are rejected, you are welcome to use the low-level API.

This policy is aimed to propagate best practises when interpreting EDID
and DisplayID information which can often be cryptic or even inconsistent.

libdisplay-info uses [semantic versioning]. The public API is not yet stable.

## Contributing

Open issues and merge requests on the [GitLab project]. Discuss and ask
questions in the [#wayland] IRC channel on OFTC.

In general, the [Wayland contribution guidelines] should be followed. In
particular, each commit must carry a Signed-off-by tag to denote that the
submitter adheres to the [Developer Certificate of Origin 1.1]. This project
follows the [freedesktop.org Contributor Covenant].

## Specifications

Both EDID and DisplayID are defined by VESA and the specifications are publicly
available. There are multiple versions defined and they have multiple
mechanisms for extending the base specification. Some of the extensions also
have mechanisms for additional extensions. This sometimes makes it hard to find
where a particular data structure is defined.

The raw data is usually read from the Display Data Channel (DDC) but other
methods of delivery exist.

Available freely [from VESA directly] are the base EDID specifications, VESA
specified EDID extensions, as well as the base DisplayID specifications. EDID
also specifies how DisplayID can be embedded in EDID.

* VESA Enhanced Extended Display Identification Data (E-EDID) Standard;
  Release A, Revision 1; February 9, 2000; Defines EDID Structure Version 1,
  Revision 3
* VESA Enhanced Extended Display Identification Data (E-EDID) Standard;
  Release A, Revision 2; September 25, 2006; Defines EDID Structure Version 1,
  Revision 4
* VESA Video Timing Block Extension Data (VTB-EXT) Standard; Release A;
  November 24, 2003
* VESA Enhanced EDID Localized String Extension Standard; Release A;
  July 10, 2003
* VESA Display Transfer Characteristics Data Block (DTCDB) Standard;
  Version 1.0; August 31, 2006
* VESA Display Information Extension Block (DI-EXT) Standard; Release A;
  August 21, 2001
* VESA Display Device Data Block (DDDB) Standard; Version 1;
  September 25, 2006
* VESA Display Identification Data (DisplayID) Standard; Version 1.3;
  July 5, 2013
* VESA Display Identification Data (DisplayID) Standard; Version 2.0;
  11 September, 2017
* VESA Display Identification Data (DisplayID) Standard; Version 2.1;
  18 November, 2021
* VESA Display Identification Data (DisplayID) Standard; Version 2.1a;
  18 March, 2024

The most common extension to both EDID and DisplayID is CTA-861 which can be
downloaded for free [from CTA].

* A DTV Profile for Uncompressed High Speed Digital Interfaces
  (CTA-861-E, CTA-861-F, CTA-861-G, CTA-861-H, CTA-861-I)
* HDR Static Metadata Extensions (CTA-861.3-A)

The CTA-861 specification allows for Vendor Specific Data Blocks (VSDB). Their
specifications are often proprietary and have to be reverse engineered. Some
specifications are available publicly.

* HDMI and HDMI Forum VSDB are specified in the HDMI specification
* Microsoft EDID Extension for [HMDs and Specialized Monitors]

The libdisplay-info structures of the low-level API always point to the
relevant specification and section therein.

## Building

libdisplay-info has the following dependencies:

- [hwdata] for the PNP ID database used at build-time only.

libdisplay-info is built using [Meson]:

    meson setup build/
    ninja -C build/

## Testing

The low-level EDID library is tested against [edid-decode]. `test/data/`
contains a small collection of EDID blobs and diffs between upstream
`edid-decode` and our `di-edid-decode` clone. Our CI ensures the diffs are
up-to-date. A patch should never make the diffs grow larger. To re-generate the
test data, build `edid-decode` at the Git revision mentioned in
`.gitlab-ci.yml`, put the executable in `PATH`, and run
`ninja -C build/ gen-test-data`.

The latest code coverage report is available on [GitLab CI][coverage].

## Fuzzing

To fuzz libdisplay-info with [AFL], the library needs to be instrumented:

    CC=afl-gcc meson build/
    ninja -C build/
    afl-fuzz -i test/data/ -o afl/ build/di-edid-decode/di-edid-decode

[website]: https://emersion.pages.freedesktop.org/libdisplay-info/
[semantic versioning]: https://semver.org/
[GitLab project]: https://gitlab.freedesktop.org/emersion/libdisplay-info
[#wayland]: ircs://irc.oftc.net/#wayland
[Wayland contribution guidelines]: https://gitlab.freedesktop.org/wayland/wayland/-/blob/main/CONTRIBUTING.md
[Developer Certificate of Origin 1.1]: https://developercertificate.org/
[freedesktop.org Contributor Covenant]: https://www.freedesktop.org/wiki/CodeOfConduct/
[from VESA directly]: https://vesa.org/vesa-standards/
[from CTA]: https://www.cta.tech/SearchResults?search=CTA-861
[HMDs and Specialized Monitors]: https://learn.microsoft.com/en-us/windows-hardware/drivers/display/specialized-monitors-edid-extension
[hwdata]: https://github.com/vcrhonek/hwdata
[Meson]: https://mesonbuild.com/
[coverage]: https://gitlab.freedesktop.org/emersion/libdisplay-info/-/jobs/artifacts/main/file/build/meson-logs/coveragereport/index.html?job=build-gcc
[edid-decode]: https://git.linuxtv.org/edid-decode.git/
[AFL]: https://lcamtuf.coredump.cx/afl/
