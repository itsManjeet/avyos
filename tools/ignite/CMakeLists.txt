cmake_minimum_required(VERSION 3.28)

project(ignite
        VERSION "1.0"
        DESCRIPTION "Build tool for avyos"
        LANGUAGES C CXX)

set(CMAKE_CXX_STANDARD 20)
set(CMAKE_CXX_STANDARD_REQUIRED ON)

find_package(Threads REQUIRED)

add_executable(${PROJECT_NAME}
        main.cpp
        Ignite.cpp Ignite.h
        Container.cpp Container.h
        Configuration.cpp Configuration.h
        Recipe.cpp Recipe.h
        Executor.cpp Executor.h
        Dashboard.cpp Dashboard.h)

target_link_libraries(${PROJECT_NAME} PRIVATE
        yaml-cpp
        Threads::Threads)

install(DIRECTORY dashboard/
        DESTINATION share/ignite/dashboard
        FILES_MATCHING
        PATTERN "*.html"
        PATTERN "*.css"
        PATTERN "*.js")