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

#include "Configuration.h"
#include <filesystem>
#include <map>
#include <string>
#include <vector>

struct Recipe {
    std::string id, version, about;
    std::string integration, cache;

    std::vector<std::string> depends, backup;
    Configuration config;

    std::vector<std::string> build_time_depends, sources;
    std::string element_id;

    Recipe() = default;

    virtual ~Recipe() = default;

    explicit Recipe(const std::string& filepath,
            const std::filesystem::path& search_path = {});

    void update_from_data(const std::string& data, const std::string& filepath);

    void update_from_file(const std::string& filepath);

    [[nodiscard]] std::string name() const {
        auto name = this->id;
        for (auto& c : name)
            if (c == '/') c = '-';
        return name;
    }

    [[nodiscard]] virtual std::string str() const {
        std::stringstream ss;
        ss << "id: " << id << "\n";

        ss << "version: " << version << "\n"
           << "about: " << about << "\n"
           << "cache: " << cache << "\n";

        if (!depends.empty()) {
            ss << "depends:\n";
            for (auto const& i : depends)
                ss << "- " << std::filesystem::path(i).replace_extension()
                   << "\n";
        }

        if (!backup.empty()) {
            ss << "backup:\n";
            for (auto const& i : backup) ss << "- " << i << "\n";
        }

        if (!integration.empty()) {
            ss << "script: |-\n";
            std::string line;
            std::stringstream script(integration);
            while (std::getline(script, line)) { ss << "  " << line << '\n'; }
            ss << std::endl;
        }

        return ss.str();
    }

    [[nodiscard]] std::string package_name(std::string eid = "") const {
        if (eid.empty()) eid = id;
        for (auto& c : eid)
            if (c == '/') c = '-';
        return eid + "-" + version + "-" + cache + ".pkg";
    }

    static std::string resolve(const std::string& data,
            const std::map<std::string, std::string>& variables);

    void resolve(const Configuration& global,
            const std::map<std::string, std::string>& extra = {}) {
        for (auto& source : sources) {
            source = resolve(source, global, extra);
        }
    }

    [[nodiscard]] std::string resolve(const std::string& value,
            const Configuration& global,
            const std::map<std::string, std::string>& extra = {}) const;
};