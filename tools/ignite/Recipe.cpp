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

#include "Recipe.h"

#include <fstream>
#include <regex>

Recipe::Recipe(
        const std::string& filepath, const std::filesystem::path& search_path) {
    config.search_path.push_back(search_path);
    config.node["cache"] = "none";
    update_from_file(filepath);
    if (config.node["build-depends"]) {
        for (auto const& dep : config.node["build-depends"]) {
            build_time_depends.push_back(dep.as<std::string>());
        };
    }
    if (config.node["sources"]) {
        for (auto const& dep : config.node["sources"])
            sources.emplace_back(dep.as<std::string>());
    }
    element_id = std::filesystem::path(
            filepath.substr((search_path / "elements").string().size() + 1))
                         .replace_extension();
}

void Recipe::update_from_data(
        const std::string& data, const std::string& filepath) {
    config.update_from(data, filepath);

    id = config.get<std::string>("id");
    version = config.get<std::string>("version");
    about = config.get<std::string>("about", "");
    cache = config.get<std::string>("cache");

    if (config.node["depends"]) {
        for (auto const& dep : config.node["depends"]) {
            depends.emplace_back(dep.as<std::string>());
        }
    }
    if (config.node["backup"]) {
        for (auto const& b : config.node["backup"])
            backup.push_back(b.as<std::string>());
    }
    if (config.node["integration"]) {
        integration = config.node["integration"].as<std::string>();
    }
}

std::vector<std::string> split(const std::string& str, char del) {
    std::stringstream ss(str);
    std::vector<std::string> l;
    for (std::string s; std::getline(ss, s, del);) { l.push_back(s); }
    return l;
}

std::string replace(std::string v, char old, char n) {
    for (auto& i : v) {
        if (i == old) i = n;
    }
    return v;
}

std::string Recipe::resolve(const std::string& data,
        const std::map<std::string, std::string>& variables) {
    std::regex pattern(R"(\%\{([^}]+)\})");
    std::smatch match;
    std::string result = data;

    while (std::regex_search(result, match, pattern)) {
        if (match.size() > 1) {
            std::string variable = match.str(1);
            auto it = variables.find(variable);
            if (it != variables.end()) {
                result.replace(match[0].first, match[0].second, it->second);
                // TODO: a better way to handle this hack
            } else if (variable.starts_with("version:")) {
                auto data = variable.substr(variable.find_first_of(':') + 1);
                auto vit = variables.find("version");
                if (vit == variables.end()) {
                    throw std::runtime_error(
                            "version variable not defined for '" + variable +
                            "'");
                }
                auto version = vit->second;
                try {
                    auto nth = std::stoi(data) - 1;
                    int count = 0, position = 0;
                    while (count <= nth) {
                        position += 1;
                        position = version.find('.', position);
                        if (position == std::string::npos) {
                            throw std::string(
                                    "invalid variable value spliting for " +
                                    std::to_string(nth) + "th position");
                        }
                        count++;
                    }
                    result.replace(match[0].first, match[0].second,
                            version.substr(0, position));
                } catch (const std::string& error) {
                    throw std::runtime_error(error);
                } catch (...) {
                    if (data.empty()) {
                        throw std::runtime_error(
                                "empty version: specifier in '" + variable +
                                "'");
                    }
                    result.replace(match[0].first, match[0].second,
                            replace(version, '.', data[0]));
                }
            } else {
                throw std::runtime_error(
                        "undefined variable '" + variable + "'");
            }
        }
    }
    return result;
}

std::string Recipe::resolve(const std::string& value,
        const Configuration& global,
        const std::map<std::string, std::string>& extra) const {
    std::map<std::string, std::string> variables;

    // Lowest priority: global config variables (defaults)
    if (global.node["variables"]) {
        for (auto const& v : global.node["variables"]) {
            variables[v.first.as<std::string>()] = v.second.as<std::string>();
        }
    }

    // Recipe scalar keys override global defaults (e.g. configure, compile)
    for (auto const& i : this->config.node) {
        if (i.second.IsScalar()) {
            try {
                variables[i.first.as<std::string>()] =
                        i.second.as<std::string>();
            } catch (...) {}
        }
    }

    // Recipe-level explicit variables override everything above
    if (this->config.node["variables"]) {
        for (auto const& v : this->config.node["variables"]) {
            variables[v.first.as<std::string>()] = v.second.as<std::string>();
        }
    }

    variables["build-dir"] = "_pkgupd_build_dir";

    // Highest priority: extra variables (build-context paths like
    // install-root, build-root) must not be overridden by config
    for (auto const& [k, v] : extra) { variables[k] = v; }

    return resolve(value, variables);
}

void Recipe::update_from_file(const std::string& filepath) {
    std::ifstream reader(filepath);
    if (!reader.good())
        throw std::runtime_error("failed to read file '" + filepath + "'");
    update_from_data(std::string((std::istreambuf_iterator<char>(reader)),
                             (std::istreambuf_iterator<char>())),
            filepath);
}
