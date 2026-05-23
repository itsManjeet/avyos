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

#include "Configuration.h"

#include <filesystem>
#include <fstream>

static YAML::Node merge(const YAML::Node& a, const YAML::Node& b) {
    if (a.IsNull()) return YAML::Clone(b);

    if (a.IsMap() && b.IsMap()) {
        YAML::Node merged = YAML::Clone(a);
        for (auto const& i : b) {
            if (auto const key = i.first.as<std::string>(); a[key]) {
                merged[key] = merge(a[key], i.second);
            } else {
                merged[key] = YAML::Clone(i.second);
            }
        }
        return merged;
    }

    if (a.IsSequence() && b.IsSequence()) {
        YAML::Node merged = YAML::Clone(a);
        for (const auto& elem : b) { merged.push_back(YAML::Clone(elem)); }
        return merged;
    }

    if (a.IsScalar() && b.IsScalar()) { return YAML::Clone(a); }

    std::stringstream ss;
    ss << a;
    throw std::runtime_error("Can't handle other type: " + ss.str());
}

void Configuration::update_from_file(const std::string& filepath) {
    std::ifstream reader(filepath);
    if (!reader.good()) {
        throw std::runtime_error("failed to read file '" + filepath + "'");
    }
    std::string content((std::istreambuf_iterator<char>(reader)),
            (std::istreambuf_iterator<char>()));
    update_from(content, filepath);
}

void Configuration::update_from(
        const std::string& data, const std::string& filepath) {
    auto new_node = YAML::Load(data);
    node = merge(node, new_node);
    if (new_node["merge"]) {
        for (auto const& i : new_node["merge"]) {
            try {
                auto path = std::filesystem::path(filepath).parent_path() /
                            i.as<std::string>();
                if (std::filesystem::exists(path)) {
                    update_from_file(
                            std::filesystem::path(filepath).parent_path() /
                            i.as<std::string>());
                } else {
                    bool found = false;
                    for (auto const& p : search_path) {
                        if (std::filesystem::exists(p / i.as<std::string>())) {
                            update_from_file(p / i.as<std::string>());
                            found = true;
                            break;
                        }
                    }
                    if (!found) {
                        throw std::runtime_error(
                                "missing required file to merge '" +
                                i.as<std::string>() + "'");
                    }
                }
            } catch (const std::exception& exception) {
                throw std::runtime_error("failed to load " + filepath +
                                         " because " + exception.what() +
                                         " to merge");
            }
        }
    }
}
