cmake_minimum_required(VERSION 3.28)
project(avyos
        VERSION "2.0"
        DESCRIPTION "Immutable OpenSource OS"
        LANGUAGES C CXX ASM)

set(CMAKE_CXX_STANDARD 23)
set(CMAKE_CXX_STANDARD_REQUIRED ON)

add_subdirectory(system)