version: 6.0.5

configure: >-
  -DCMAKE_BUILD_TYPE=Release
  -DBUILD_TESTING=OFF
  -Wno-dev

sources:
  - https://download.kde.org/stable/plasma/%{version}/%{id}-%{version}.tar.xz
