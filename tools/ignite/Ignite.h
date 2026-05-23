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
#include "Container.h"
#include "Recipe.h"
#include <filesystem>
#include <map>
#include <optional>
#include <string>
#include <vector>

struct Compiler {
    std::string file;
    std::string script;
};

class Ignite {
    std::map<std::string, Recipe> pool;
    std::filesystem::path project_path, cache_path;

    std::map<std::string, Compiler> compilers;
    std::map<std::string, std::string> hash_cache;

public:
    Configuration& config;

    using State = std::tuple<std::string, Recipe, bool>;

    explicit Ignite(Configuration& config, std::filesystem::path project_path,
            std::filesystem::path cache_path, const std::string& arch);

    void load();

    [[nodiscard]] std::filesystem::path const& get_cache_path() const {
        return cache_path;
    }

    std::string hash(const Recipe& build_info);

    std::filesystem::path cachefile(const Recipe& build_info);

    [[nodiscard]] std::filesystem::path workspace_path(
            const Recipe& build_info) const;

    [[nodiscard]] bool workspace_available(const Recipe& build_info) const;

    [[nodiscard]] std::filesystem::path workspace_cachefile(
            const Recipe& build_info) const;

    void workspace_init(const Recipe& build_info);

    void workspace_finish(const Recipe& build_info);

    void fetch_sources(const std::vector<std::string>& ids = {},
            bool force = false);

    [[nodiscard]] std::map<std::string, Recipe> const& get_pool() const {
        return pool;
    }

    [[nodiscard]] std::map<std::string, Recipe>& get_pool() { return pool; }

    void resolve(const std::vector<std::string>& id, std::vector<State>& output,
            bool devel = true, bool include_depends = true,
            bool include_extra = true);

    enum class ContainerType {
        Build,
        Shell,
    };

    Container setup_container(const Recipe& build_info,
            ContainerType container_type = ContainerType::Shell);

    void integrate(Container& container, const Recipe& build_info,
            const std::filesystem::path& root = {});

    void build(const Recipe& build_info);

    std::optional<std::filesystem::path> prepare_sources(
            const Recipe& build_info, Container* container,
            const std::filesystem::path& source_dir,
            const std::filesystem::path& build_root);

    void fetch_source_file(const std::string& source,
            const std::filesystem::path& source_dir, bool force = false);

    void verify_source_file(const std::filesystem::path& filepath);

    std::optional<std::filesystem::path> prepare_workspace_sources(
            const Recipe& build_info,
            const std::filesystem::path& build_root);

    Compiler get_compiler(const Recipe& build_info, Container* container,
            const std::filesystem::path& build_root);

    void compile_source(const Recipe& build_info, Container* container,
            const std::filesystem::path& build_root,
            const std::filesystem::path& install_root);

    void pack(const Recipe& build_info, Container* container,
            const std::filesystem::path& install_root,
            const std::filesystem::path& package);

    void strip(const Recipe& build_info, Container* container,
            const std::filesystem::path& install_root);
};
