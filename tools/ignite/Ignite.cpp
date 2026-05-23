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

#include "Ignite.h"

#include "Executor.h"
#include "picosha2.h"
#include <algorithm>
#include <cctype>
#include <filesystem>
#include <fstream>
#include <functional>
#include <iomanip>
#include <iostream>
#include <regex>
#include <sstream>
#include <utility>
#include <unistd.h>

namespace {

std::string workspace_component_id(const Recipe& recipe) {
    auto component_id =
            recipe.element_id.empty() ? recipe.id : recipe.element_id;
    for (auto& c : component_id) {
        if (c == '/' || c == '\\') c = '-';
    }
    return component_id;
}

std::string workspace_package_name(const Recipe& recipe) {
    auto package_name = recipe.package_name(recipe.element_id);
    if (package_name.ends_with(".pkg")) {
        package_name.resize(package_name.size() - 4);
    }
    return package_name + "-workspace.pkg";
}

std::vector<std::string> quilt_environment() {
    return {
            "HOME=/",
            "PATH=/usr/bin:/bin:/usr/local/bin",
            "QUILT_PATCHES=.ignite-workspace/patches",
            "QUILT_SERIES=series",
            "QUILT_PC=.pc",
    };
}

std::filesystem::path quilt_binary() {
    for (auto const& path : {"/usr/bin/quilt", "/bin/quilt",
                 "/usr/local/bin/quilt"}) {
        if (std::filesystem::exists(path)) { return path; }
    }
    throw std::runtime_error(
            "quilt not found; install quilt to finish workspaces");
}

std::filesystem::path diff_binary() {
    for (auto const& path : {"/usr/bin/diff", "/bin/diff",
                 "/usr/local/bin/diff"}) {
        if (std::filesystem::exists(path)) { return path; }
    }
    throw std::runtime_error(
            "diff not found; install diffutils to finish workspaces");
}

bool is_workspace_metadata(const std::filesystem::path& relative_path) {
    auto it = relative_path.begin();
    if (it == relative_path.end()) { return false; }
    return *it == ".ignite-workspace" || *it == ".pc";
}

void copy_workspace_tree(const std::filesystem::path& source,
        const std::filesystem::path& target) {
    std::filesystem::create_directories(target);

    for (auto iter = std::filesystem::recursive_directory_iterator(source);
            iter != std::filesystem::recursive_directory_iterator(); ++iter) {
        auto relative_path = std::filesystem::relative(iter->path(), source);
        if (is_workspace_metadata(relative_path)) {
            if (iter->is_directory()) { iter.disable_recursion_pending(); }
            continue;
        }

        auto target_path = target / relative_path;
        if (iter->is_symlink()) {
            std::filesystem::create_directories(target_path.parent_path());
            std::filesystem::remove(target_path);
            std::filesystem::copy_symlink(iter->path(), target_path);
        } else if (iter->is_directory()) {
            std::filesystem::create_directories(target_path);
        } else if (iter->is_regular_file()) {
            std::filesystem::create_directories(target_path.parent_path());
            std::filesystem::copy_file(iter->path(), target_path,
                    std::filesystem::copy_options::overwrite_existing);
            std::filesystem::permissions(
                    target_path, iter->status().permissions());
        }
    }
}

void move_directory_contents(const std::filesystem::path& source,
        const std::filesystem::path& target) {
    std::vector<std::filesystem::path> entries;
    for (auto const& entry : std::filesystem::directory_iterator(source)) {
        entries.push_back(entry.path());
    }

    for (auto const& entry : entries) {
        auto target_path = target / entry.filename();
        if (std::filesystem::exists(target_path)) {
            throw std::runtime_error("workspace source collision at '" +
                                     target_path.string() + "'");
        }
        std::filesystem::rename(entry, target_path);
    }
}

std::string patch_safe_name(std::string value) {
    if (value.empty()) { value = "workspace"; }
    for (auto& c : value) {
        if (!(std::isalnum(static_cast<unsigned char>(c)) || c == '-' ||
                    c == '_' || c == '.')) {
            c = '-';
        }
    }
    return value;
}

std::string numbered_patch_name(int number, const std::string& tail) {
    std::stringstream ss;
    ss << std::setw(4) << std::setfill('0') << number << '-' << tail;
    return ss.str();
}

std::filesystem::path next_patch_path(const std::filesystem::path& output_dir,
        const std::filesystem::path& preferred_name) {
    auto preferred_path = output_dir / preferred_name;
    if (!std::filesystem::exists(preferred_path)) { return preferred_path; }

    static const std::regex numbered_name(R"(^([0-9]{4})-(.+)$)");
    auto filename = preferred_name.filename().string();
    std::smatch match;
    int number = 1;
    std::string tail = filename;
    if (std::regex_match(filename, match, numbered_name)) {
        number = std::stoi(match.str(1)) + 1;
        tail = match.str(2);
    }

    while (true) {
        auto candidate = output_dir / numbered_patch_name(number++, tail);
        if (!std::filesystem::exists(candidate)) { return candidate; }
    }
}

std::vector<std::string> split_source_spec(const std::string& source) {
    std::vector<std::string> parts;
    std::size_t start = 0;
    while (true) {
        auto end = source.find("::", start);
        if (end == std::string::npos) {
            parts.push_back(source.substr(start));
            break;
        }
        parts.push_back(source.substr(start, end - start));
        start = end + 2;
    }

    return parts;
}

struct SourceSpec {
    std::string filename;
    std::string url;
    bool noextract = false;
};

SourceSpec parse_source_spec(const std::string& source) {
    auto parts = split_source_spec(source);
    std::vector<std::string> values;
    SourceSpec spec;

    for (auto const& part : parts) {
        if (part == "noextract") {
            spec.noextract = true;
        } else {
            values.push_back(part);
        }
    }

    if (values.empty()) {
        throw std::runtime_error("source has no url: '" + source + "'");
    }

    spec.url = values.back();
    if (values.size() > 1) {
        spec.filename = values.front();
    } else {
        spec.filename = std::filesystem::path(spec.url).filename().string();
    }

    return spec;
}

std::string source_filename(const std::string& source) {
    return parse_source_spec(source).filename;
}

std::string source_url(const std::string& source) {
    return parse_source_spec(source).url;
}

bool source_noextract(const std::string& source) {
    return parse_source_spec(source).noextract;
}

std::string file_sha256(const std::filesystem::path& filepath) {
    if (!std::filesystem::is_regular_file(filepath)) {
        throw std::runtime_error(
                "cannot checksum non-regular source '" + filepath.string() +
                "'");
    }

    std::ifstream reader(filepath, std::ios::binary);
    if (!reader.good()) {
        throw std::runtime_error("failed to read source '" + filepath.string() +
                                 "' for checksum");
    }

    std::string hash_sum;
    picosha2::hash256_hex_string(
            std::istreambuf_iterator<char>(reader),
            std::istreambuf_iterator<char>(), hash_sum);
    return hash_sum;
}

std::map<std::string, std::string> read_checksum_lock(
        const std::filesystem::path& lock_file) {
    std::map<std::string, std::string> checksums;
    std::ifstream reader(lock_file);
    if (!reader.good()) {
        throw std::runtime_error("failed to read checksum lock '" +
                                 lock_file.string() + "'");
    }

    for (std::string line; std::getline(reader, line);) {
        if (auto comment = line.find('#'); comment != std::string::npos) {
            line = line.substr(0, comment);
        }

        std::stringstream ss(line);
        std::string hash;
        std::string filename;
        if (!(ss >> hash >> filename)) { continue; }
        checksums[filename] = hash;
    }

    return checksums;
}

void write_checksum_lock(const std::filesystem::path& lock_file,
        const std::map<std::string, std::string>& checksums) {
    std::ofstream writer(lock_file);
    if (!writer.good()) {
        throw std::runtime_error("failed to write checksum lock '" +
                                 lock_file.string() + "'");
    }

    for (auto const& [filename, checksum] : checksums) {
        writer << checksum << "  " << filename << '\n';
    }
}

std::vector<std::filesystem::path> read_quilt_series(
        const std::filesystem::path& series_file) {
    std::ifstream reader(series_file);
    if (!reader.good()) {
        throw std::runtime_error("failed to read quilt series file '" +
                                 series_file.string() + "'");
    }

    std::vector<std::filesystem::path> patches;
    for (std::string line; std::getline(reader, line);) {
        if (auto comment = line.find('#'); comment != std::string::npos) {
            line = line.substr(0, comment);
        }

        std::stringstream ss(line);
        std::string patch;
        if (ss >> patch) { patches.emplace_back(patch); }
    }
    return patches;
}

void make_tree_removable(const std::filesystem::path& root) {
    if (!std::filesystem::exists(root)) return;

    for (auto const& i : std::filesystem::recursive_directory_iterator(root)) {
        if (access(i.path().c_str(), W_OK) != 0) {
            std::error_code code;
            std::filesystem::permissions(i.path(),
                    std::filesystem::perms::owner_all |
                            std::filesystem::perms::group_all |
                            std::filesystem::perms::others_all,
                    code);
        }
    }
}

bool remove_tree(const std::filesystem::path& root) {
    make_tree_removable(root);
    std::error_code code;
    std::filesystem::remove_all(root, code);

    if (std::filesystem::exists(root)) {
        Executor("/bin/rm").arg("-r").arg("-f").arg(root).run();
    }

    return !std::filesystem::exists(root);
}

} // namespace

Ignite::Ignite(Configuration& config, std::filesystem::path project_path,
        std::filesystem::path cache_path, const std::string& arch)
        : config{config}, project_path(std::move(project_path)),
          cache_path(std::move(cache_path)) {
    auto config_file = this->project_path / ("config-" + arch + ".yml");
    if (!std::filesystem::exists(config_file)) {
        throw std::runtime_error("failed to load configuration file '" +
                                 config_file.string() + "'");
    }
    config.update_from_file(config_file);

    if (config.node["compiler"]) {
        for (auto const& c : config.node["compiler"]) {
            compilers[c.first.as<std::string>()] = Compiler{
                    c.second["file"].as<std::string>(),
                    c.second["script"].as<std::string>(),
            };
        }
    }
}

void Ignite::load() {
    auto external_path = project_path / "elements";
    for (auto const& i :
            std::filesystem::recursive_directory_iterator(external_path)) {
        if (i.is_regular_file() && i.path().has_extension() &&
                i.path().extension() == ".yml") {
            auto element_path =
                    std::filesystem::relative(i.path(), external_path);
            try {
                pool[element_path.string()] = Recipe(i.path(), project_path);
            } catch (const std::exception& exception) {
                throw std::runtime_error("failed to load '" +
                                         element_path.string() + " because " +
                                         exception.what());
            }
        }
    }
    std::cout << "Ignite::load(): Loaded " << pool.size() << " elements\n";
}

void Ignite::resolve(const std::vector<std::string>& id,
        std::vector<State>& output, bool devel, bool include_depends,
        bool include_extra) {
    std::map<std::string, bool> visited;

    std::function<void(const std::string& i)> dfs = [&](const std::string& i) {
        visited[i] = true;
        auto recipe = pool.find(i);
        if (recipe == pool.end()) { throw std::runtime_error("MISSING " + i); }

        auto depends = recipe->second.depends;
        if (devel) {
            depends.insert(depends.end(),
                    recipe->second.build_time_depends.begin(),
                    recipe->second.build_time_depends.end());
        }
        if (include_extra) {
            if (recipe->second.config.node["include"]) {
                for (auto const& i : recipe->second.config.node["include"]) {
                    depends.push_back(i.as<std::string>());
                }
            }
        }

        if (include_depends) {
            for (const auto& depend : depends) {
                if (visited[depend]) continue;
                try {
                    dfs(depend);
                } catch (const std::exception& exception) {
                    throw std::runtime_error(std::string(exception.what()) +
                                             "\n\tTRACEBACK " + i);
                }
            }
        }

        auto resolved_recipe = recipe->second;
        resolved_recipe.cache = hash(resolved_recipe);
        auto cached = !workspace_available(resolved_recipe) &&
                      std::filesystem::exists(cachefile(resolved_recipe));

        for (auto depend : depends) {
            auto idx = std::find_if(output.begin(), output.end(),
                    [&depend](const auto& val) -> bool {
                        return std::get<0>(val) == depend;
                    });
            if (idx == output.end()) {
                if (auto in_pool = pool.find(depend); in_pool == pool.end()) {
                    throw std::runtime_error("internal error " + depend +
                                             " not in a pool for " + i);
                } else {
                    auto local_recipe = in_pool->second;
                    local_recipe.cache = hash(local_recipe);
                    if (workspace_available(local_recipe) ||
                            !std::filesystem::exists(cachefile(local_recipe))) {
                        cached = false;
                        break;
                    }
                }
            } else {
                if (!std::get<2>(*idx)) {
                    cached = false;
                    break;
                }
            }
        }
        output.emplace_back(i, resolved_recipe, cached);
    };

    for (auto const& i : id) { dfs(i); }
}

std::string Ignite::hash(const Recipe& recipe) {
    const auto cache_key =
            recipe.element_id.empty() ? recipe.id : recipe.element_id;
    if (auto it = hash_cache.find(cache_key); it != hash_cache.end()) {
        return it->second;
    }

    std::string hash_sum;

    {
        std::stringstream ss;
        ss << recipe.config.node;
        picosha2::hash256_hex_string(ss.str(), hash_sum);
    }

    std::vector<std::string> includes;
    if (recipe.config.node["include"]) {
        for (auto const& i : recipe.config.node["include"]) {
            includes.push_back(i.as<std::string>());
        }
    }

    for (auto const& d :
            {recipe.depends, recipe.build_time_depends, includes}) {
        for (auto const& i : d) {
            {
                auto depend_recipe = pool.find(i);
                if (depend_recipe == pool.end()) {
                    throw std::runtime_error("missing required element '" + i +
                                             " for " + recipe.id);
                }
                // Recursively hash the dependency so transitive changes
                // propagate: if a dep-of-dep changes, this recipe's hash
                // changes too.
                auto dep_hash = hash(depend_recipe->second);
                picosha2::hash256_hex_string(dep_hash + hash_sum, hash_sum);
            }
        }
    }

    hash_cache[cache_key] = hash_sum;
    return hash_sum;
}

void Ignite::build(const Recipe& recipe) {
    auto container = setup_container(recipe, ContainerType::Build);
    std::shared_ptr<void> _(nullptr, [&container](...) {
        remove_tree(container.host_root);
    });
    std::filesystem::create_directories(cache_path / "logs");
    std::ofstream logger(cache_path / "logs" /
                         (recipe.package_name(recipe.element_id) + ".log"));
    container.logger = &logger;

    auto package_path = cachefile(recipe);
    std::optional<std::filesystem::path> subdir;
    if (workspace_available(recipe)) {
        std::cout << "Ignite::build(): using workspace "
                  << workspace_path(recipe) << std::endl;
        subdir = prepare_workspace_sources(
                recipe, container.host_root / "build-root");
    } else {
        subdir = prepare_sources(recipe, &container, cache_path / "sources",
                container.host_root / "build-root");
    }
    if (!subdir) subdir = ".";

    auto build_root =
            std::filesystem::path("build-root") /
            recipe.config.get<std::string>("build-dir", subdir->string());
    build_root = recipe.resolve(build_root.string(), config);
    try {
        compile_source(recipe, &container, build_root, "install-root");
        pack(recipe, &container, container.host_root / "install-root",
                package_path);
    } catch (const std::exception& exception) {
        std::cout << "ERROR: " << exception.what() << std::endl;
        Executor("/bin/sh").container(&container).interactive().run();
        throw;
    }
}

Container Ignite::setup_container(
        const Recipe& recipe, const ContainerType container_type) {
    auto env = std::vector<std::string>{"NOCONFIGURE=1", "HOME=/",
            "SHELL=/bin/sh", "TERM=dumb", "USER=nishu", "LOGNAME=nishu",
            "LC_ALL=C", "TZ=UTC", "SOURCE_DATE_EPOCH=918239400",
            "PKGSYSTEM_ENABLE_FSYNC=0"};
    if (auto n = config.node["environ"]; n) {
        for (auto const& i : n) env.push_back(i.as<std::string>());
    }
    if (auto n = recipe.config.node["environ"]; n) {
        for (auto const& i : n) env.push_back(i.as<std::string>());
    }

    auto host_root =
            (cache_path / "temp" / recipe.package_name(recipe.element_id));
    if (!remove_tree(host_root)) {
        throw std::runtime_error(
                "failed to clean stale build root '" + host_root.string() + "'");
    }
    std::filesystem::create_directories(host_root);

    std::vector<std::string> capabilities;
    if (recipe.config.node["capabilities"]) {
        for (auto const& i : recipe.config.node["capabilities"]) {
            capabilities.push_back(i.as<std::string>());
        }
    }

    auto container = Container{
            .environ = env,
            .binds =
                    {
                            {"/sources", cache_path / "sources"},
                            {"/cache", cache_path / "cache"},
                            {"/files", project_path / "files"},
                            {"/patches", project_path / "patches"},
                            {"/avyos", project_path},

                    },
            .capabilities = capabilities,
            .host_root = host_root,
            .base_dir = project_path,
            .name = recipe.package_name(recipe.element_id),
    };
    for (auto const& i : {"sources", "cache"}) {
        std::filesystem::create_directories(cache_path / i);
    }
    config.node["dir.build"] = host_root.string();

    // TODO: temporary fix for glib and dependent packages to resolve
    // -Werror=missing-include-dir
    std::filesystem::create_directories(
            host_root / "usr" / "local" / "include");

    std::vector<State> states;
    auto depends = recipe.depends;
    if (container_type == ContainerType::Build) {
        depends.insert(depends.end(), recipe.build_time_depends.begin(),
                recipe.build_time_depends.end());
    }

    resolve(depends, states, true, true, false);
    for (auto const& [path, info, cached] : states) {
        integrate(container, info, "");
    }

    if (container_type == ContainerType::Shell) {
        integrate(container, recipe, "");
    }

    // Add Included elements to provided path
    if (recipe.config.node["include"]) {
        states.clear();

        std::vector<std::string> include;
        for (auto const& i : recipe.config.node["include"]) {
            include.push_back(recipe.resolve(i.as<std::string>(), config));
        }

        resolve(include, states, false,
                recipe.config.get<bool>("include-depends", true), false);

        if (recipe.config.node["include-upon"]) {
            std::vector<State> sub_states;
            resolve({recipe.config.node["include-upon"].as<std::string>()},
                    sub_states, false, true, false);
            states.erase(
                    std::remove_if(states.begin(), states.end(),
                            [&sub_states](const State& state) -> bool {
                                return std::find_if(sub_states.begin(),
                                               sub_states.end(),
                                               [&state](
                                                       const State& other_state)
                                                       -> bool {
                                                   return std::get<0>(state) ==
                                                          std::get<0>(
                                                                  other_state);
                                               }) != sub_states.end();
                            }),
                    states.end());
        }

        for (auto const& [path, info, cached] : states) {
            auto installation_path = std::filesystem::path("install-root") /
                                     recipe.package_name(recipe.element_id);
            installation_path = recipe.config.get<std::string>(
                    recipe.name() + "-include-path",
                    recipe.config.get<std::string>(
                            "include-root", installation_path.string()));
            integrate(container, info, installation_path);
        }
    }

    return container;
}

std::filesystem::path Ignite::cachefile(const Recipe& recipe) {
    if (workspace_available(recipe)) { return workspace_cachefile(recipe); }
    return cache_path / "cache" / recipe.package_name(recipe.element_id);
}

std::filesystem::path Ignite::workspace_path(const Recipe& recipe) const {
    return cache_path / "workspaces" / workspace_component_id(recipe);
}

bool Ignite::workspace_available(const Recipe& recipe) const {
    auto path = workspace_path(recipe);
    return std::filesystem::is_directory(path) &&
           std::filesystem::exists(path / ".ignite-workspace" / "metadata");
}

std::filesystem::path Ignite::workspace_cachefile(const Recipe& recipe) const {
    return cache_path / "cache" / workspace_package_name(recipe);
}

void Ignite::integrate(Container& container, const Recipe& recipe,
        const std::filesystem::path& root) {
    auto container_root =
            container.host_root /
            (root.has_root_path()
                            ? std::filesystem::path(root.string().substr(1))
                            : root);
    std::cout << "Ignite::integrate(" << recipe.package_name() << ")\n";
    std::filesystem::create_directories(container_root);

    auto cache_file_path = cachefile(recipe);
    try {
        auto extractor = Executor("/bin/tar")
                                 .arg("-xPhf")
                                 .arg(cache_file_path)
                                 .arg("-C")
                                 .arg(container_root);

        if (root.empty()) {
            extractor.arg("--exclude=./etc/hosts")
                    .arg("--exclude=./etc/hostname")
                    .arg("--exclude=./etc/resolv.conf")
                    .arg("--exclude=./proc")
                    .arg("--exclude=./run")
                    .arg("--exclude=./sys")
                    .arg("--exclude=./dev");
        }

        extractor.execute();
    } catch (const std::exception& exception) {
        throw std::runtime_error("failed to integrate " +
                                 recipe.package_name(recipe.element_id) + " " +
                                 exception.what());
    }

    if (root.empty()) {
        if (!recipe.integration.empty()) {
            auto integration_script =
                    recipe.resolve(recipe.integration, config);
            Executor("/bin/sh")
                    .arg("-ec")
                    .arg(integration_script)
                    .container(&container)
                    .execute();
        }
    } else {
        auto meta_info = recipe;
        auto data_dir = container_root / "usr" / "share" / "pkgupd" /
                        "manifest" / meta_info.package_name();
        std::filesystem::create_directories(data_dir);
        std::cout << "Iginite::integrate::save_data(" << recipe.package_name()
                  << ")@" << meta_info.package_name() << "\n";
        {
            std::ofstream writer(data_dir / "info");
            writer << meta_info.str();
        }
        if (!recipe.integration.empty()) {
            auto integration_script =
                    recipe.resolve(recipe.integration, config);
            {
                std::ofstream writer(data_dir / "integration");
                writer << integration_script;
            }
        }

        std::ofstream writer(data_dir / "files");
        int status = Executor("/bin/tar")
                             .arg("-tf")
                             .arg(cache_file_path)
                             .arg("--exclude=./etc/hosts")
                             .arg("--exclude=./etc/hostname")
                             .arg("--exclude=./etc/resolv.conf")
                             .arg("--exclude=./proc")
                             .arg("--exclude=./run")
                             .arg("--exclude=./sys")
                             .arg("--exclude=./dev")
                             .start()
                             .wait(&writer);
        writer.close();
        if (status != 0) {
            throw std::runtime_error("failed to read tar files from " +
                                     cache_file_path.string());
        }
    }
}

void extract(const std::filesystem::path& filepath,
        const std::string& output_path, std::vector<std::string>& files_list) {
    std::stringstream output;
    if (!std::filesystem::exists(output_path)) {
        std::error_code code;
        std::filesystem::create_directories(output_path, code);
        if (code) {
            throw std::runtime_error("failed to create required directory '" +
                                     output_path + "': " + code.message());
        }
    }

    auto exe = "/bin/tar";
    if (filepath.has_extension() && filepath.extension() == ".zip") {
        exe = "/bin/bsdtar";
    }

    int status = Executor(exe)
                         .arg("-xvf")
                         .arg(filepath)
                         .arg("-C")
                         .arg(output_path)
                         .start()
                         .wait(&output);

    std::stringstream ss(output.str());
    for (std::string f; std::getline(ss, f);) {
        if (f.starts_with("./")) f = f.substr(2);
        if (f.starts_with("x ")) f = f.substr(2);
        if (f.empty()) continue;
        files_list.emplace_back(f);
    }

    if (status != 0) {
        throw std::runtime_error(
                "failed to extract " + filepath.string() + " :" + output.str());
    }
}

bool is_archive(const std::filesystem::path& filepath) {
    for (auto const& ext : {".tar", ".zip", ".gz", ".xz", ".bzip2", ".tgz",
                 ".txz", ".bz2", ".zst", ".zstd", ".lz"}) {
        if (filepath.has_extension() && filepath.extension() == ext) {
            return true;
        }
    }
    return false;
}

void Ignite::fetch_source_file(const std::string& source,
        const std::filesystem::path& source_dir, bool force) {
    auto url = source_url(source);
    auto filename = source_filename(source);
    auto filepath = source_dir / filename;
    auto temp_filepath = std::filesystem::path(filepath.string() + ".tmp");

    std::filesystem::create_directories(source_dir);
    if (force) {
        std::filesystem::remove(filepath);
        std::filesystem::remove(temp_filepath);
    } else if (std::filesystem::exists(filepath)) {
        return;
    }

    if (url.starts_with("http")) {
        Executor("/bin/wget")
                .arg("-c")
                .arg(url)
                .arg("-O")
                .arg(temp_filepath)
                .execute();
        std::filesystem::rename(temp_filepath, filepath);
    } else {
        if (!force && std::filesystem::exists(filepath)) { return; }
        std::filesystem::copy(project_path / url, filepath,
                std::filesystem::copy_options::recursive |
                        std::filesystem::copy_options::overwrite_existing);
    }
}

void Ignite::verify_source_file(const std::filesystem::path& filepath) {
    auto lock_file = project_path / "checksum.lock";
    if (!std::filesystem::exists(lock_file)) { return; }

    auto checksums = read_checksum_lock(lock_file);
    auto filename = filepath.filename().string();
    auto checksum = checksums.find(filename);
    if (checksum == checksums.end()) {
        throw std::runtime_error("checksum.lock has no entry for source '" +
                                 filename + "'");
    }

    auto actual = file_sha256(filepath);
    if (actual != checksum->second) {
        throw std::runtime_error("checksum mismatch for source '" + filename +
                                 "': expected " + checksum->second +
                                 ", got " + actual);
    }
}

void Ignite::fetch_sources(const std::vector<std::string>& ids, bool force) {
    std::vector<Recipe> recipes;
    if (ids.empty()) {
        for (auto const& [id, recipe] : pool) { recipes.push_back(recipe); }
    } else {
        std::vector<State> states;
        resolve(ids, states);
        for (auto& [id, recipe, cached] : states) { recipes.push_back(recipe); }
    }

    auto source_dir = cache_path / "sources";
    auto lock_file = project_path / "checksum.lock";
    std::map<std::string, std::string> checksums;
    if (!force && std::filesystem::exists(lock_file)) {
        checksums = read_checksum_lock(lock_file);
    }

    for (auto& recipe : recipes) {
        try {
            recipe.resolve(config);
            for (auto const& source : recipe.sources) {
                auto filename = source_filename(source);
                auto filepath = source_dir / filename;
                fetch_source_file(source, source_dir, force);

                auto locked = checksums.find(filename);
                if (!force && locked != checksums.end()) {
                    std::cout << "Using locked source: " << filename
                              << std::endl;
                    continue;
                }

                auto actual = file_sha256(filepath);
                checksums[filename] = actual;
                std::cout << "Fetched source: " << filename << std::endl;
            }
        } catch (const std::exception& exception) {
            auto element =
                    recipe.element_id.empty() ? recipe.id : recipe.element_id;
            throw std::runtime_error("failed to fetch sources for '" + element +
                                     "' (" + recipe.id + "): " +
                                     exception.what());
        }
        write_checksum_lock(lock_file, checksums);
        std::cout << "Checkpointed checksum lock after: "
                  << recipe.element_id << std::endl;
    }

    std::cout << "Wrote checksum lock: " << lock_file << std::endl;
}

std::optional<std::filesystem::path> Ignite::prepare_sources(
        const Recipe& build_info, Container* container,
        const std::filesystem::path& source_dir,
        const std::filesystem::path& build_root) {
    std::optional<std::filesystem::path> subdir;

    std::filesystem::create_directories(build_root);

    for (auto url : build_info.sources) {
        auto filename = source_filename(url);
        auto filepath = source_dir / filename;
        fetch_source_file(url, source_dir);
        verify_source_file(filepath);

        if (is_archive(filepath) && !source_noextract(url)) {
            std::vector<std::string> files_list;

            extract(filepath,
                    build_root / (subdir ? *subdir : std::filesystem::path("")),
                    files_list);
            if (!subdir && !files_list.empty()) {
                std::string dir = files_list.front();
                auto idx = dir.find('/');
                if (idx != std::string::npos) { dir = dir.substr(0, idx); }
                subdir = dir;
            }
        } else {
            std::filesystem::copy_file(filepath,
                    build_root /
                            (subdir ? *subdir : std::filesystem::path("")) /
                            filename,
                    std::filesystem::copy_options::overwrite_existing);
        }
    }
    return subdir;
}

std::optional<std::filesystem::path> Ignite::prepare_workspace_sources(
        const Recipe& build_info, const std::filesystem::path& build_root) {
    auto workspace = workspace_path(build_info);
    if (!workspace_available(build_info)) {
        throw std::runtime_error("workspace is not available for '" +
                                 build_info.id + "'");
    }

    copy_workspace_tree(workspace, build_root);
    return ".";
}

void Ignite::workspace_init(const Recipe& build_info) {
    auto workspace = workspace_path(build_info);
    if (std::filesystem::exists(workspace)) {
        throw std::runtime_error("workspace already exists at '" +
                                 workspace.string() + "'");
    }

    std::filesystem::create_directories(cache_path / "workspaces");
    std::filesystem::create_directories(cache_path / "sources");

    auto temp_workspace = workspace;
    temp_workspace += ".tmp." + std::to_string(getpid());
    for (int suffix = 0; std::filesystem::exists(temp_workspace); ++suffix) {
        temp_workspace = workspace;
        temp_workspace += ".tmp." + std::to_string(getpid()) + "." +
                          std::to_string(suffix);
    }

    try {
        auto subdir = prepare_sources(build_info, nullptr,
                cache_path / "sources", temp_workspace);
        if (subdir && !subdir->empty() && *subdir != ".") {
            auto source_root = temp_workspace / *subdir;
            if (std::filesystem::is_directory(source_root)) {
                move_directory_contents(source_root, temp_workspace);
                std::filesystem::remove(source_root);
            }
        }

        auto metadata_dir = temp_workspace / ".ignite-workspace";
        auto patches_dir = metadata_dir / "patches";
        auto original_dir = metadata_dir / "original";
        std::filesystem::create_directories(patches_dir);
        copy_workspace_tree(temp_workspace, original_dir);

        {
            std::ofstream writer(metadata_dir / "metadata");
            writer << "id: " << build_info.id << '\n'
                   << "element: " << build_info.element_id << '\n'
                   << "version: " << build_info.version << '\n';
        }
        {
            std::ofstream writer(metadata_dir / "env");
            writer << "export QUILT_PATCHES=.ignite-workspace/patches\n"
                   << "export QUILT_SERIES=series\n"
                   << "export QUILT_PC=.pc\n";
        }
        {
            std::ofstream writer(patches_dir / "series", std::ios::app);
        }

        std::filesystem::rename(temp_workspace, workspace);
    } catch (...) {
        std::filesystem::remove_all(temp_workspace);
        throw;
    }

    std::cout << "Workspace initialized: " << workspace << '\n'
              << "Use quilt with: source .ignite-workspace/env" << std::endl;
}

void Ignite::workspace_finish(const Recipe& build_info) {
    auto workspace = workspace_path(build_info);
    if (!workspace_available(build_info)) {
        throw std::runtime_error("workspace is not available for '" +
                                 build_info.id + "'");
    }

    auto metadata_dir = workspace / ".ignite-workspace";
    auto patches_dir = metadata_dir / "patches";
    auto series_file = patches_dir / "series";

    auto patches = read_quilt_series(series_file);
    if (!patches.empty()) {
        auto quilt = quilt_binary();
        auto [applied_status, applied_output] =
                Executor(quilt.string())
                        .arg("applied")
                        .path(workspace)
                        .environ(quilt_environment())
                        .output();
        if (applied_status == 0 && !applied_output.empty()) {
            Executor(quilt.string())
                    .arg("refresh")
                    .path(workspace)
                    .environ(quilt_environment())
                    .execute();
        }
    }

    auto output_dir = project_path / "patches" / build_info.id;
    std::filesystem::create_directories(output_dir);

    if (patches.empty()) {
        auto diff_root = cache_path / "temp" /
                         ("workspace-diff-" +
                                 workspace_component_id(build_info) + "-" +
                                 std::to_string(getpid()));
        std::filesystem::remove_all(diff_root);
        std::filesystem::create_directories(diff_root);
        std::shared_ptr<void> cleanup(nullptr, [&diff_root](...) {
            std::filesystem::remove_all(diff_root);
        });

        auto original_dir = metadata_dir / "original";
        if (std::filesystem::is_directory(original_dir)) {
            copy_workspace_tree(original_dir, diff_root / "a");
        } else {
            auto subdir = prepare_sources(build_info, nullptr,
                    cache_path / "sources", diff_root / "a");
            if (subdir && !subdir->empty() && *subdir != ".") {
                auto source_root = diff_root / "a" / *subdir;
                if (std::filesystem::is_directory(source_root)) {
                    move_directory_contents(source_root, diff_root / "a");
                    std::filesystem::remove(source_root);
                }
            }
        }
        copy_workspace_tree(workspace, diff_root / "b");

        auto [status, diff_output] =
                Executor(diff_binary().string())
                        .arg("-Naur")
                        .arg("a")
                        .arg("b")
                        .path(diff_root)
                        .output();
        if (status > 1) {
            throw std::runtime_error(
                    "failed to generate workspace diff: " + diff_output);
        }
        if (diff_output.empty()) {
            std::cout << "Workspace has no source changes; no patches exported"
                      << std::endl;
            std::filesystem::remove_all(workspace);
            std::cout << "Workspace closed: " << workspace << std::endl;
            return;
        }

        auto output_patch = next_patch_path(output_dir,
                "0001-" + patch_safe_name(build_info.id) +
                        "-workspace.patch");
        {
            std::ofstream writer(output_patch);
            writer << diff_output;
            if (!diff_output.ends_with('\n')) { writer << '\n'; }
        }
        std::cout << "Exported patch: " << output_patch << std::endl;
        std::filesystem::remove_all(workspace);
        std::cout << "Workspace closed: " << workspace << std::endl;
        return;
    }

    for (auto const& patch : patches) {
        auto source_patch = patches_dir / patch;
        if (!std::filesystem::exists(source_patch)) {
            throw std::runtime_error("quilt patch listed in series is missing: " +
                                     source_patch.string());
        }

        auto output_name = source_patch.filename();
        if (output_name.extension() != ".patch") { output_name += ".patch"; }
        auto output_patch = next_patch_path(output_dir, output_name);

        std::filesystem::copy_file(source_patch, output_patch,
                std::filesystem::copy_options::none);
        std::cout << "Exported patch: " << output_patch << std::endl;
    }

    std::filesystem::remove_all(workspace);
    std::cout << "Workspace closed: " << workspace << std::endl;
}

void Ignite::compile_source(const Recipe& build_info, Container* container,
        const std::filesystem::path& build_root,
        const std::filesystem::path& install_root) {
    std::vector<std::string> env;
    if (config.node["environ"]) {
        for (auto const& e : config.node["environ"]) {
            env.push_back(e.as<std::string>());
        }
    }

    if (build_info.config.node["environ"]) {
        for (auto const& e : build_info.config.node["environ"]) {
            env.push_back(e.as<std::string>());
        }
    }
    std::map<std::string, std::string> extra_variables;

    auto resolved_install_root =
            (container ? container->host_root : std::filesystem::path("")) /
            install_root / build_info.package_name();
    auto resolved_build_root =
            (container ? container->host_root : std::filesystem::path("")) /
            build_root;
    extra_variables["install-root"] = std::filesystem::path("/") /
                                      install_root / build_info.package_name();
    extra_variables["build-root"] = std::filesystem::path("/") / build_root;

    if (auto pre_script = build_info.config.get<std::string>("pre-script", "");
            !pre_script.empty()) {
        pre_script = build_info.resolve(pre_script, config, extra_variables);
        std::cout << "Exec(pre-script)" << std::endl;

        Executor("/bin/sh")
                .arg("-ec")
                .arg(pre_script)
                .path(extra_variables["build-root"])
                .environ(env)
                .container(container)
                .execute();
    }

    if (build_info.config.get<std::string>("build-type", "") == "import") {
        auto source = resolved_build_root /
                      std::filesystem::path(
                              build_info.config.get<std::string>("source", ""));
        auto target = resolved_install_root /
                      std::filesystem::path(
                              build_info.config.get<std::string>("target", ""));
        std::filesystem::create_directories(target);
        Executor("/bin/cp")
                .arg("-rap")
                .arg(source / ".")
                .arg("-t")
                .arg(target)
                .execute();
    } else {
        auto script = build_info.config.get<std::string>("script", "");
        if (script.empty()) {
            auto compiler =
                    get_compiler(build_info, container, resolved_build_root);
            script = compiler.script;
        }

        script = build_info.resolve(script, config, extra_variables);

        std::cout << "Exec(script)" << std::endl;

        if (script.length() > 500) {
            auto script_path = resolved_build_root / "pkgupd_exec_script.sh";
            {
                std::ofstream script_writer(script_path);
                script_writer << script;
            }

            Executor("/bin/sh")
                    .arg("-e")
                    .arg("pkgupd_exec_script.sh")
                    .path(extra_variables["build-root"])
                    .environ(env)
                    .container(container)
                    .execute();
        } else {
            Executor("/bin/sh")
                    .arg("-ec")
                    .arg(script)
                    .path(extra_variables["build-root"])
                    .environ(env)
                    .container(container)
                    .execute();
        }
    }

    if (auto post_script =
                    build_info.config.get<std::string>("post-script", "");
            !post_script.empty()) {
        post_script = build_info.resolve(post_script, config, extra_variables);
        std::cout << "Exec(post-script)" << std::endl;

        Executor("/bin/sh")
                .arg("-ec")
                .arg(post_script)
                .path(extra_variables["build-root"])
                .environ(env)
                .container(container)
                .execute();
    }

    if (build_info.config.get<bool>("strip", true)) {
        strip(build_info, container, resolved_install_root);
    }
}

void Ignite::strip(const Recipe& build_info, Container* container,
        const std::filesystem::path& install_root) {
    std::vector<std::string> mime_to_strip;
    if (config.node["strip-mimetype"]) {
        for (auto const& i : config.node["strip-mimetype"]) {
            mime_to_strip.emplace_back(i.as<std::string>());
        }
    }
    if (build_info.config.node["strip-mimetype"]) {
        for (auto const& i : build_info.config.node["strip-mimetype"]) {
            mime_to_strip.emplace_back(i.as<std::string>());
        }
    }

    for (auto const& iter :
            std::filesystem::recursive_directory_iterator(install_root)) {
        if (!iter.is_regular_file()) continue;
        // if file is executable and writable or
        // if file ends with .so and .a
        if (((iter.path().has_extension() &&
                     (iter.path().extension() == ".so" ||
                             iter.path().extension() == ".a" ||
                             iter.path().filename().string().find(".so.") !=
                                     std::string::npos)) ||
                    (access(iter.path().c_str(), X_OK) == 0)) &&
                access(iter.path().c_str(), W_OK) == 0) {
            auto [status, mime_type] = Executor("/bin/file")
                                               .arg("-b")
                                               .arg("--mime-type")
                                               .arg(iter.path())
                                               .output();
            if (status != 0) {
                std::cerr << "failed to read MIME TYPE for " +
                                     iter.path().string() + ": " + mime_type
                          << std::endl;
                continue;
            }

            if (std::find(mime_to_strip.begin(), mime_to_strip.end(),
                        mime_type) == mime_to_strip.end()) {
                continue;
            }

            try {
                auto dbg_file_path = iter.path().string() + ".dbg";
                // Copy debugging symbols to dbg directory
                Executor("/bin/objcopy")
                        .arg("--only-keep-debug")
                        .arg(iter.path())
                        .arg(dbg_file_path)
                        .silent()
                        .execute();

                auto fname = iter.path().filename().string();
                std::string strip_args = "--strip-all";
                if (iter.path().has_extension() &&
                        iter.path().extension() == ".a") {
                    strip_args = "--strip-debug";
                } else if ((iter.path().has_extension() &&
                                   iter.path().extension() == ".so") ||
                           fname.find(".so.") != std::string::npos) {
                    strip_args = "--strip-unneeded";
                }

                // Strip out the debugging symbols
                Executor("/bin/strip")
                        .arg(strip_args)
                        .arg(iter.path())
                        .silent()
                        .execute();

                // Link to the extracted debugging symbols
                Executor("/bin/objcopy")
                        .arg("--add-gnu-debuglink=" +
                                iter.path().filename().string() + ".dbg")
                        .arg(iter.path())
                        .path(iter.path().parent_path())
                        .silent()
                        .execute();
            } catch (const std::exception& exception) {
                std::cerr << "failed to strip " << iter.path().string()
                          << " with mimetype " << mime_type << " because "
                          << exception.what() << std::endl;
                continue;
            }
        }
    }
}

void Ignite::pack(const Recipe& build_info, Container* container,
        const std::filesystem::path& install_root,
        const std::filesystem::path& package) {
    auto install_root_package = install_root / build_info.package_name();
    auto install_root_dbg = install_root / (build_info.package_name() + ".dbg");

    for (auto const& i : {install_root_dbg}) {
        std::filesystem::create_directories(i);
    }

    std::vector<std::regex> keep_files;
    if (build_info.config.node["keep-files"]) {
        for (auto const& i : build_info.config.node["keep-files"]) {
            keep_files.emplace_back(i.as<std::string>());
        }
    }

    auto keep_file = [&keep_files](const std::string& filename) -> bool {
        for (auto const& i : keep_files) {
            if (std::regex_match(filename, i)) { return true; }
        }
        return false;
    };

    auto replace_directory = [&](const std::filesystem::path& filepath,
                                     const std::filesystem::path& old_parent,
                                     const std::filesystem::path& new_parent)
            -> std::filesystem::path {
        auto relative_path = std::filesystem::relative(filepath, old_parent);
        return new_parent / relative_path;
    };

    auto move_file = [&](const std::filesystem::path& filepath,
                             const std::filesystem::path& new_path) {
        auto replaced_path =
                replace_directory(filepath, install_root_package, new_path);
        std::filesystem::create_directories(replaced_path.parent_path());
        std::filesystem::rename(filepath, replaced_path);
    };

    for (auto const& dbg : {"usr/src", "usr/lib/debug"}) {
        if (auto path = install_root_package / dbg;
                std::filesystem::exists(path)) {
            move_file(path, install_root_dbg);
        }
    }

    // Collect paths first; modifying the tree during
    // recursive_directory_iterator is undefined behavior.
    std::vector<std::filesystem::path> dirs_to_clean;
    std::vector<std::filesystem::path> files_to_remove;
    std::vector<std::filesystem::path> dbg_to_move;
    bool clean_empty_dirs =
            build_info.config.get<bool>("clean-empty-dir", true);

    for (auto const& i : std::filesystem::recursive_directory_iterator(
                 install_root_package)) {
        if (i.is_directory()) {
            if (clean_empty_dirs) { dirs_to_clean.push_back(i.path()); }
        } else if (!keep_files.empty() && keep_file(i.path().filename())) {
            continue;
        } else if (i.path().has_extension() && i.path().extension() == ".la") {
            files_to_remove.push_back(i.path());
        } else if (i.path().has_extension() && i.path().extension() == ".dbg") {
            dbg_to_move.push_back(i.path());
        }
    }

    for (auto const& p : files_to_remove) { std::filesystem::remove(p); }
    for (auto const& p : dbg_to_move) { move_file(p, install_root_dbg); }
    // Reverse so children are removed before parents, allowing cascading
    // cleanup.
    std::reverse(dirs_to_clean.begin(), dirs_to_clean.end());
    for (auto const& p : dirs_to_clean) {
        if (std::filesystem::exists(p) && std::filesystem::is_empty(p)) {
            std::filesystem::remove(p);
        }
    }

    std::cout << "Compressing " << build_info.name() << std::endl;

    std::ofstream user_map(install_root / "user-map");
    user_map << "+" << getuid() << " root:0\n"
             << config.get<std::string>("user-map", "") << '\n'
             << build_info.config.get<std::string>("user-map", "") << '\n';
    user_map.close();

    std::ofstream group_map(install_root / "group-map");
    group_map << "+" << getgid() << " root:0\n"
              << config.get<std::string>("group-map", "") << '\n'
              << build_info.config.get<std::string>("group-map", "") << '\n';
    group_map.close();

    auto compressor = config.get<std::string>(
            "package-compressor", std::string("zstd -T0 -1"));

    for (auto const& i : std::map<std::string, std::string>{
                 {"", install_root_package},
                 {".dbg", install_root_dbg},
         }) {
        Executor("/bin/tar")
                .arg("--use-compress-program=" + compressor)
                .arg("--owner-map=" + (install_root / "user-map").string())
                .arg("--group-map=" + (install_root / "group-map").string())
                .arg("-cPf")
                .arg(package.string() + i.first)
                .arg("-C")
                .arg(i.second)
                .arg(".")
                .execute();
    }
}

Compiler Ignite::get_compiler(const Recipe& build_info, Container* container,
        const std::filesystem::path& build_root) {
    std::string build_type;
    if (build_info.config.node["build-type"]) {
        build_type = build_info.config.node["build-type"].as<std::string>();
    } else {
        for (auto const& [id, compiler] : compilers) {
            if (std::filesystem::exists(build_root / compiler.file)) {
                build_type = id;
                break;
            }
        }
    }

    if (build_type.empty() || !compilers.contains(build_type)) {
        throw std::runtime_error(
                "unknown build-type or failed to detect build-type '" +
                build_type + "' at " + build_root.string());
    }
    return compilers[build_type];
}
