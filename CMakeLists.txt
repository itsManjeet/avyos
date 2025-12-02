cmake_minimum_required(VERSION 3.30)
project(avyos
        VERSION "3.0"
        DESCRIPTION "Immutable OpenSource OS"
        LANGUAGES C CXX ASM)

set(CMAKE_CXX_STANDARD 23)
set(CMAKE_CXX_STANDARD_REQUIRED ON)

add_subdirectory(tools)
