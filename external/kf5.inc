version: 6.2.0

configure: >-
  -DCMAKE_BUILD_TYPE=Release
  -DBUILD_TESTING=OFF
  -Wno-dev

sources:
  - https://download.kde.org/stable/frameworks/%{version:2}/%{id}-%{version}.tar.xz
