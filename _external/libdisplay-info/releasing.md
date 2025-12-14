# Releasing

To make a release of libdisplay-info, follow these steps.

0. Verify the test suites and codebase checks pass.  All of the tests should
   either pass or skip.

       ninja -C build/ test

1. Update the first stanza of `meson.build` to the intended version.

   Then commit your changes:

       RELEASE_NUMBER="x.y.z"
       RELEASE_NAME="[alpha|beta|RC1|RC2|official|point]"
       git status
       git commit meson.build -m "build: bump to version $RELEASE_NUMBER for the $RELEASE_NAME release"
       git push

2. Run the `release.sh` script to generate the tarballs, sign and upload them,
   and generate a release announcement template.

3. Compose the release announcements.  The script will generate a
   libdisplay-info-x.y.z.announce file with a list of changes and tags.  Prepend
   these with a human-readable listing of the most notable changes.  For x.y.0
   releases, indicate the schedule for the x.y+1.0 release.

4. PGP sign the release announcement and send it to
   <wayland-devel@lists.freedesktop.org>.

For x.y.0 releases, also create the release series x.y branch.  The x.y branch
is for bug fixes and conservative changes to the x.y.0 release, and is where we
create x.y.z releases from.  Creating the x.y branch opens up main for new
development and lets new development move on.

    git branch x.y [sha]
    git push origin x.y

For stable branches, we commit fixes to main first, then `git cherry-pick -x`
them back to the stable branch.

