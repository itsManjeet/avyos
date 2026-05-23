/*
* Copyright (c) 2024 Manjeet Singh <itsmanjeet1998@gmail.com>.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, version 3.
 *
 * This program is distributed in the hope that it will be useful, but
 * WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
 * General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program. If not, see <http://www.gnu.org/licenses/>.
 *
 */

#pragma once

#include <filesystem>
#include <string>
#include <vector>
#include <yaml-cpp/yaml.h>

struct Configuration {
    YAML::Node node;

    Configuration() = default;

    std::vector<std::filesystem::path> search_path;

    void update_from_file(const std::string& filepath);

    void update_from(const std::string& data, const std::string& filepath = {});

    template <typename T> T get(const std::string& key, T fallback) const {
        if (node[key]) { return node[key].as<T>(); }
        return fallback;
    }

    template <typename T> T get(const std::string& key) const {
        if (node[key]) { return node[key].as<T>(); }
        throw std::runtime_error("missing required key '" + key + "'");
    }

    template <typename T> void set(const std::string& key, T value) {
        node[key] = value;
    }

    template <typename T> void push(const std::string& key, T value) {
        if (!node[key]) { node[key] = {}; }
        node[key].push_back(value);
    }
};
