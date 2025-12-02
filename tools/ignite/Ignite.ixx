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

module;

#include "picosha2.h"

#include <yaml-cpp/yaml.h>

#include <filesystem>
#include <functional>
#include <iostream>
#include <map>
#include <optional>
#include <regex>
#include <string>
#include <utility>
#include <vector>

export module ignite;

export import :Recipe;
export import :Configuration;
export import :Container;
export import :Executor;

export struct Compiler
{
    std::string file;
    std::string script;
};

void extract(const std::filesystem::path& filepath,
             const std::string& output_path,
             std::vector<std::string>& files_list)
{
    std::stringstream output;
    if (!std::filesystem::exists(output_path))
    {
        std::error_code code;
        std::filesystem::create_directories(output_path, code);
        if (code)
        {
            throw std::runtime_error("failed to create required directory '" +
                                     output_path + "': " + code.message());
        }
    }

    auto exe = "/bin/tar";
    if (filepath.has_extension() && filepath.extension() == ".zip")
    {
        exe = "/bin/bsdtar";
    }

    int status = Executor(exe)
                     .arg("-xvPf")
                     .arg(filepath)
                     .arg("-C")
                     .arg(output_path)
                     .start()
                     .wait(&output);

    std::stringstream ss(output.str());
    for (std::string f; std::getline(ss, f);)
    {
        if (f.starts_with("./"))
            f = f.substr(2);
        if (f.starts_with("x "))
            f = f.substr(2);
        if (f.empty())
            continue;
        files_list.emplace_back(f);
    }

    if (status != 0)
    {
        throw std::runtime_error(
            "failed to extract " + filepath.string() + " :" + output.str());
    }
}

bool is_archive(const std::filesystem::path& filepath)
{
    for (const auto& ext : {".tar", ".zip", ".gz", ".xz", ".bzip2", ".tgz",
                            ".txz", ".bz2", ".zst", ".zstd", ".lz"})
    {
        if (filepath.has_extension() && filepath.extension() == ext)
        {
            return true;
        }
    }
    return false;
}

export class Ignite
{
    std::map<std::string, Recipe> pool;
    std::filesystem::path project_path, cache_path;

    std::map<std::string, Compiler> compilers;

  public:
    Configuration& config;

    using State = std::tuple<std::string, Recipe, bool>;

    explicit Ignite(Configuration& config, std::filesystem::path project_path,
                    std::filesystem::path cache_path, const std::string& arch) :
        config{config}, project_path(std::move(project_path)),
        cache_path(std::move(cache_path))
    {
        auto config_file = this->project_path / ("config-" + arch + ".yml");
        if (!std::filesystem::exists(config_file))
        {
            throw std::runtime_error("failed to load configuration file '" +
                                     config_file.string() + "'");
        }
        config.update_from_file(config_file);

        if (config.node["compiler"])
        {
            for (const auto& c : config.node["compiler"])
            {
                compilers[c.first.as<std::string>()] = Compiler{
                    c.second["file"].as<std::string>(),
                    c.second["script"].as<std::string>(),
                };
            }
        }
    }

    void load()
    {
        auto external_path = project_path / "external";
        for (const auto& i :
             std::filesystem::recursive_directory_iterator(external_path))
        {
            if (i.is_regular_file() && i.path().has_extension() &&
                i.path().extension() == ".yml")
            {
                auto element_path =
                    std::filesystem::relative(i.path(), external_path);
                try
                {
                    pool[element_path.string()] =
                        Recipe(i.path(), project_path);
                }
                catch (const std::exception& exception)
                {
                    throw std::runtime_error(
                        "failed to load '" + element_path.string() +
                        " because " + exception.what());
                }
            }
        }
        std::cout << "Ignite::load(): Loaded " << pool.size() << " elements\n";
    }

    [[nodiscard]] const std::filesystem::path& get_cache_path() const
    {
        return cache_path;
    }

    std::string hash(const Recipe& recipe)
    {
        std::string hash_sum;

        {
            std::stringstream ss;
            ss << recipe.config.node;
            picosha2::hash256_hex_string(ss.str(), hash_sum);
        }

        std::vector<std::string> includes;
        if (recipe.config.node["include"])
        {
            for (const auto& i : recipe.config.node["include"])
            {
                includes.push_back(i.as<std::string>());
            }
        }

        for (const auto& d :
             {recipe.depends, recipe.build_time_depends, includes})
        {
            for (const auto& i : d)
            {
                {
                    auto depend_recipe = pool.find(i);
                    if (depend_recipe == pool.end())
                    {
                        throw std::runtime_error("missing required element '" +
                                                 i + " for " + recipe.id);
                    }
                    std::stringstream ss;
                    ss << depend_recipe->second.config.node;
                    picosha2::hash256_hex_string(ss.str() + hash_sum, hash_sum);
                }
            }
        }

        return hash_sum;
    }

    std::filesystem::path cachefile(const Recipe& recipe)
    {
        return cache_path / "cache" / recipe.package_name(recipe.element_id);
    }

    [[nodiscard]] const std::map<std::string, Recipe>& get_pool() const
    {
        return pool;
    }

    [[nodiscard]] std::map<std::string, Recipe>& get_pool()
    {
        return pool;
    }

    void resolve(const std::vector<std::string>& id, std::vector<State>& output,
                 bool devel = true, bool include_depends = true,
                 bool include_extra = true)
    {
        std::map<std::string, bool> visited;

        std::function<void(const std::string& i)> dfs = [&](const std::string&
                                                                i) {
            visited[i] = true;
            auto recipe = pool.find(i);
            if (recipe == pool.end())
            {
                throw std::runtime_error("MISSING " + i);
            }

            auto depends = recipe->second.depends;
            if (devel)
            {
                depends.insert(depends.end(),
                               recipe->second.build_time_depends.begin(),
                               recipe->second.build_time_depends.end());
            }
            if (include_extra)
            {
                if (recipe->second.config.node["include"])
                {
                    for (const auto& i : recipe->second.config.node["include"])
                    {
                        depends.push_back(i.as<std::string>());
                    }
                }
            }

            if (include_depends)
            {
                for (const auto& depend : depends)
                {
                    if (visited[depend])
                        continue;
                    try
                    {
                        dfs(depend);
                    }
                    catch (const std::exception& exception)
                    {
                        throw std::runtime_error(std::string(exception.what()) +
                                                 "\n\tTRACEBACK " + i);
                    }
                }
            }

            recipe->second.cache = hash(recipe->second);
            auto cached = std::filesystem::exists(cachefile(recipe->second));

            for (auto depend : depends)
            {
                auto idx = std::find_if(output.begin(), output.end(),
                                        [&depend](const auto& val) -> bool {
                                            return std::get<0>(val) == depend;
                                        });
                if (idx == output.end())
                {
                    if (auto in_pool = pool.find(depend); in_pool == pool.end())
                    {
                        throw std::runtime_error("internal error " + depend +
                                                 " not in a pool for " + i);
                    }
                    else
                    {
                        auto local_recipe = in_pool->second;
                        local_recipe.cache = hash(local_recipe);
                        if (!std::filesystem::exists(cachefile(local_recipe)))
                        {
                            cached = false;
                            break;
                        }
                    }
                }
                else
                {
                    if (!std::get<2>(*idx))
                    {
                        cached = false;
                        break;
                    }
                }
            }
            output.emplace_back(i, recipe->second, cached);
        };

        for (const auto& i : id)
        {
            dfs(i);
        }
    }

    enum class ContainerType
    {
        Build,
        Shell,
    };

    Container setup_container(
        const Recipe& recipe,
        ContainerType container_type = ContainerType::Shell)
    {
        auto env = std::vector<std::string>{
            "NOCONFIGURE=1",     "HOME=/",     "SHELL=/bin/sh",
            "TERM=dumb",         "USER=nishu", "LOGNAME=nishu",
            "LC_ALL=C",          "TZ=UTC",     "SOURCE_DATA_EPOCH=918239400",
            "AVYOS_FILES=/files"};
        if (auto n = config.node["environ"]; n)
        {
            for (const auto& i : n)
                env.push_back(i.as<std::string>());
        }
        if (auto n = recipe.config.node["environ"]; n)
        {
            for (const auto& i : n)
                env.push_back(i.as<std::string>());
        }

        auto host_root =
            (cache_path / "temp" / recipe.package_name(recipe.element_id));
        std::filesystem::create_directories(host_root);

        std::vector<std::string> capabilities;
        if (recipe.config.node["capabilities"])
        {
            for (const auto& i : recipe.config.node["capabilities"])
            {
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
        for (const auto& i : {"sources", "cache"})
        {
            std::filesystem::create_directories(cache_path / i);
        }
        config.node["dir.build"] = host_root.string();

        // TODO: temporary fix for glib and dependent packages to resolve
        // -Werror=missing-include-dir
        std::filesystem::create_directories(
            host_root / "usr" / "local" / "include");

        std::vector<State> states;
        auto depends = recipe.depends;
        if (container_type == ContainerType::Build)
        {
            depends.insert(depends.end(), recipe.build_time_depends.begin(),
                           recipe.build_time_depends.end());
        }

        resolve(depends, states, true, true, false);
        for (const auto& [path, info, cached] : states)
        {
            integrate(container, info, "");
        }

        if (container_type == ContainerType::Shell)
        {
            integrate(container, recipe, "");
        }

        // Add Included elements to provided path
        if (recipe.config.node["include"])
        {
            states.clear();

            std::vector<std::string> include;
            for (const auto& i : recipe.config.node["include"])
            {
                include.push_back(recipe.resolve(i.as<std::string>(), config));
            }

            resolve(include, states, false,
                    recipe.config.get<bool>("include-depends", true), false);

            if (recipe.config.node["include-upon"])
            {
                std::vector<State> sub_states;
                resolve({recipe.config.node["include-upon"].as<std::string>()},
                        sub_states, false, true, false);
                states.erase(
                    std::remove_if(
                        states.begin(), states.end(),
                        [&sub_states](const State& state) -> bool {
                            return std::find_if(
                                       sub_states.begin(), sub_states.end(),
                                       [&state](
                                           const State& other_state) -> bool {
                                           return std::get<0>(state) ==
                                                  std::get<0>(other_state);
                                       }) != sub_states.end();
                        }),
                    states.end());
            }

            for (const auto& [path, info, cached] : states)
            {
                auto installation_path = std::filesystem::path("install-root") /
                                         recipe.package_name();
                installation_path = recipe.config.get<std::string>(
                    recipe.name() + "-include-path",
                    recipe.config.get<std::string>("include-root",
                                                   installation_path.string()));
                integrate(container, info, installation_path);
            }
        }

        return container;
    }

    void integrate(Container& container, const Recipe& recipe,
                   const std::filesystem::path& root = {})
    {
        auto container_root =
            container.host_root /
            (root.has_root_path()
                 ? std::filesystem::path(root.string().substr(1))
                 : root);
        std::cout << "Ignite::integrate(" << recipe.package_name() << ")\n";
        std::filesystem::create_directories(container_root);

        auto cache_file_path = cachefile(recipe);
        try
        {
            auto extractor = Executor("/bin/tar")
                                 .arg("-xPhf")
                                 .arg(cache_file_path)
                                 .arg("-C")
                                 .arg(container_root);

            if (root.empty())
            {
                extractor.arg("--exclude=./etc/hosts")
                    .arg("--exclude=./etc/hostname")
                    .arg("--exclude=./etc/resolve.conf")
                    .arg("--exclude=./proc")
                    .arg("--exclude=./run")
                    .arg("--exclude=./sys")
                    .arg("--exclude=./dev");
            }

            extractor.execute();
        }
        catch (const std::exception& exception)
        {
            throw std::runtime_error("failed to integrate " +
                                     recipe.package_name(recipe.element_id) +
                                     " " + exception.what());
        }

        if (root.empty())
        {
            if (!recipe.integration.empty())
            {
                auto integration_script =
                    recipe.resolve(recipe.integration, config);
                Executor("/bin/sh")
                    .arg("-ec")
                    .arg(integration_script)
                    .container(&container)
                    .execute();
            }
        }
        else
        {
            auto meta_info = recipe;
            auto data_dir = container_root / "usr" / "share" / "pkgupd" /
                            "manifest" / meta_info.package_name();
            std::filesystem::create_directories(data_dir);
            std::cout << "Iginite::integrate::save_data("
                      << recipe.package_name() << ")@"
                      << meta_info.package_name() << "\n";
            {
                std::ofstream writer(data_dir / "info");
                writer << meta_info.str();
            }
            if (!recipe.integration.empty())
            {
                auto integration_script =
                    recipe.resolve(recipe.integration, config);
                {
                    std::ofstream writer(data_dir / "integration");
                    writer << integration_script;
                }
            }

            std::ofstream writer(data_dir / "files");
            int status =
                Executor("/bin/tar")
                    .arg("-tf")
                    .arg(cache_file_path)
                    .arg("--exclude=./etc/hosts")
                    .arg("--exclude=./etc/hostname")
                    .arg("--exclude=./etc/resolve.conf")
                    .arg("--exclude=./proc")
                    .arg("--exclude=./run")
                    .arg("--exclude=./sys")
                    .arg("--exclude=./dev")
                    .start()
                    .wait(&writer);
            writer.close();
            if (status != 0)
            {
                throw std::runtime_error("failed to read tar files from " +
                                         cache_file_path.string());
            }
        }
    }

    void build(const Recipe& recipe)
    {
        auto container = setup_container(recipe, ContainerType::Build);
        std::shared_ptr<void> _(nullptr, [&container](...) {
            for (const auto& i : std::filesystem::recursive_directory_iterator(
                     container.host_root))
            {
                if (access(i.path().c_str(), W_OK) != 0)
                {
                    std::error_code code;
                    std::filesystem::permissions(
                        i.path(),
                        std::filesystem::perms::group_all |
                            std::filesystem::perms::owner_all |
                            std::filesystem::perms::group_all,
                        code);
                }
            }
            std::filesystem::remove_all(container.host_root);
            // TODO: std:filesystem failed to clean container host_root
            // completely????
            Executor("/bin/rm")
                .arg("-r")
                .arg("-f")
                .arg(container.host_root)
                .execute();
        });
        std::ofstream logger(cache_path / "logs" /
                             (recipe.package_name(recipe.element_id) + ".log"));
        container.logger = &logger;

        auto package_path = cachefile(recipe);
        auto subdir =
            prepare_sources(recipe, &container, cache_path / "sources",
                            container.host_root / "build-root");
        if (!subdir)
            subdir = ".";

        auto build_root =
            std::filesystem::path("build-root") /
            recipe.config.get<std::string>("build-dir", subdir->string());
        build_root = recipe.resolve(build_root.string(), config);
        try
        {
            compile_source(recipe, &container, build_root, "install-root");
            pack(recipe, &container, container.host_root / "install-root",
                 package_path);
        }
        catch (const std::exception& exception)
        {
            std::cout << "ERROR: " << exception.what() << std::endl;
            Executor("/bin/sh").container(&container).execute();
            throw;
        }
    }

    std::optional<std::filesystem::path> prepare_sources(
        const Recipe& build_info, Container* container,
        const std::filesystem::path& source_dir,
        const std::filesystem::path& build_root)
    {
        std::optional<std::filesystem::path> subdir;

        std::filesystem::create_directories(build_root);

        for (auto url : build_info.sources)
        {
            auto filename = std::filesystem::path(url).filename().string();
            if (auto idx = url.find("::"); idx != std::string::npos)
            {
                filename = url.substr(0, idx);
                url = url.substr(idx + 2);
            }

            auto filepath = source_dir / filename;
            if (!std::filesystem::exists(filepath))
            {
                if (url.starts_with("http"))
                {
                    Executor("/bin/wget")
                        .arg(url)
                        .arg("-O")
                        .arg(filepath.string() + ".tmp")
                        .execute();
                    std::filesystem::rename(filepath.string() + ".tmp",
                                            filepath);
                }
                else
                {
                    std::filesystem::copy(
                        project_path / url, filepath,
                        std::filesystem::copy_options::recursive |
                            std::filesystem::copy_options::overwrite_existing);
                }
            }
            if (is_archive(filepath))
            {
                std::vector<std::string> files_list;

                extract(filepath,
                        build_root /
                            (subdir ? *subdir : std::filesystem::path("")),
                        files_list);
                if (!subdir)
                {
                    std::string dir = files_list.front();
                    auto idx = dir.find('/');
                    if (idx != std::string::npos)
                    {
                        dir = dir.substr(0, idx);
                    }
                    subdir = dir;
                }
            }
            else
            {
                std::filesystem::copy_file(
                    filepath,
                    build_root /
                        (subdir ? *subdir : std::filesystem::path("")) /
                        filename,
                    std::filesystem::copy_options::overwrite_existing);
            }
        }
        return subdir;
    }

    Compiler get_compiler(const Recipe& build_info, Container* container,
                          const std::filesystem::path& build_root)
    {
        std::string build_type;
        if (build_info.config.node["build-type"])
        {
            build_type = build_info.config.node["build-type"].as<std::string>();
        }
        else
        {
            for (const auto& [id, compiler] : compilers)
            {
                if (std::filesystem::exists(build_root / compiler.file))
                {
                    build_type = id;
                    break;
                }
            }
        }

        if (build_type.empty() || !compilers.contains(build_type))
        {
            throw std::runtime_error(
                "unknown build-type or failed to detect build-type '" +
                build_type + "' at " + build_root.string());
        }
        return compilers[build_type];
    }

    void compile_source(const Recipe& build_info, Container* container,
                        const std::filesystem::path& build_root,
                        const std::filesystem::path& install_root)
    {
        std::vector<std::string> env;
        if (config.node["environ"])
        {
            for (const auto& e : config.node["environ"])
            {
                env.push_back(e.as<std::string>());
            }
        }

        if (build_info.config.node["environ"])
        {
            for (const auto& e : build_info.config.node["environ"])
            {
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
        extra_variables["install-root"] =
            std::filesystem::path("/") / install_root /
            build_info.package_name();
        extra_variables["build-root"] = std::filesystem::path("/") / build_root;

        if (auto pre_script =
                build_info.config.get<std::string>("pre-script", "");
            !pre_script.empty())
        {
            pre_script =
                build_info.resolve(pre_script, config, extra_variables);
            std::cout << "Exec(pre-script)" << std::endl;

            Executor("/bin/sh")
                .arg("-ec")
                .arg(pre_script)
                .path(extra_variables["build-root"])
                .environ(env)
                .container(container)
                .execute();
        }

        if (build_info.config.get<std::string>("build-type", "") == "import")
        {
            auto source = resolved_build_root /
                          build_info.config.get<std::string>("source", "");
            auto target = resolved_install_root /
                          build_info.config.get<std::string>("target", "");
            std::filesystem::create_directories(target);
            Executor("/bin/cp")
                .arg("-rap")
                .arg(source / ".")
                .arg("-t")
                .arg(target)
                .execute();
        }
        else
        {
            auto script = build_info.config.get<std::string>("script", "");
            if (script.empty())
            {
                auto compiler =
                    get_compiler(build_info, container, resolved_build_root);
                script = compiler.script;
            }

            script = build_info.resolve(script, config, extra_variables);

            std::cout << "Exec(script)" << std::endl;
            std::cout << "Exec(pre-script)" << std::endl;

            if (script.length() > 500)
            {
                auto script_path =
                    resolved_build_root / "pkgupd_exec_script.sh";
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
            }
            else
            {
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
            !post_script.empty())
        {
            post_script =
                build_info.resolve(post_script, config, extra_variables);
            std::cout << "Exec(post-script)" << std::endl;

            Executor("/bin/sh")
                .arg("-ec")
                .arg(post_script)
                .path(extra_variables["build-root"])
                .environ(env)
                .container(container)
                .execute();
        }

        if (build_info.config.get<bool>("strip", true))
        {
            strip(build_info, container, resolved_install_root);
        }
    }

    void pack(const Recipe& build_info, Container* container,
              const std::filesystem::path& install_root,
              const std::filesystem::path& package)
    {
        auto install_root_package = install_root / build_info.package_name();
        auto install_root_dbg =
            install_root / (build_info.package_name() + ".dbg");

        for (const auto& i : {install_root_dbg})
        {
            std::filesystem::create_directories(i);
        }

        std::vector<std::regex> keep_files;
        if (build_info.config.node["keep-files"])
        {
            for (const auto& i : build_info.config.node["keep-files"])
            {
                keep_files.emplace_back(i.as<std::string>());
            }
        }

        auto keep_file = [&keep_files](const std::string& filename) -> bool {
            for (const auto& i : keep_files)
            {
                if (std::regex_match(filename, i))
                {
                    return true;
                }
            }
            return false;
        };

        auto replace_directory = [&](const std::filesystem::path& filepath,
                                     const std::filesystem::path& old_parent,
                                     const std::filesystem::path& new_parent)
            -> std::filesystem::path {
            auto relative_path =
                std::filesystem::relative(filepath, old_parent);
            return new_parent / relative_path;
        };

        auto move_file = [&](const std::filesystem::path& filepath,
                             const std::filesystem::path& new_path) {
            auto replaced_path =
                replace_directory(filepath, install_root_package, new_path);
            std::filesystem::create_directories(replaced_path.parent_path());
            std::filesystem::rename(filepath, replaced_path);
        };

        for (const auto& dbg : {"usr/src", "usr/lib/debug"})
        {
            if (auto path = install_root_package / dbg;
                std::filesystem::exists(path))
            {
                move_file(path, install_root_dbg);
            }
        }

        for (const auto& i : std::filesystem::recursive_directory_iterator(
                 install_root_package))
        {
            if (i.is_directory())
            {
                if (i.path().empty() &&
                    build_info.config.get<bool>("clean-empty-dir", true))
                {
                    std::filesystem::remove(i.path());
                }
            }
            else if (!keep_files.empty() && keep_file(i.path().filename()))
            {
                continue;
            }
            else if (i.path().has_extension() && i.path().extension() == ".la")
            {
                std::filesystem::remove(i.path());
            }
            else if (i.path().has_extension() && i.path().extension() == ".dbg")
            {
                move_file(i.path(), install_root_dbg);
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
                  << build_info.config.get<std::string>("group-map", "")
                  << '\n';
        group_map.close();

        for (const auto& i : std::map<std::string, std::string>{
                 {"", install_root_package},
                 {".dbg", install_root_dbg},
             })
        {
            Executor("/bin/tar")
                .arg("--zstd")
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

    void strip(const Recipe& build_info, Container* container,
               const std::filesystem::path& install_root)
    {
        for (const auto& iter :
             std::filesystem::recursive_directory_iterator(install_root))
        {
            if (!iter.is_regular_file())
                continue;
            // if file is executable and writable or
            // if file ends with .so and .a
            // TODO check if it cover all cases
            if (((iter.path().has_extension() &&
                  (iter.path().extension() == ".so" ||
                   iter.path().extension() == ".a")) ||
                 (access(iter.path().c_str(), X_OK) == 0)) &&
                access(iter.path().c_str(), W_OK) == 0)
            {
                auto [status, mime_type] =
                    Executor("/bin/file")
                        .arg("-b")
                        .arg("--mime-type")
                        .arg(iter.path())
                        .output();
                if (status != 0)
                {
                    std::cerr << "failed to read MIME TYPE for " +
                                     iter.path().string() + ": " + mime_type
                              << std::endl;
                    continue;
                }

                std::vector<std::string> mime_to_strip;
                if (config.node["strip-mimetype"])
                {
                    for (const auto& i : config.node["strip-mimetype"])
                    {
                        mime_to_strip.emplace_back(i.as<std::string>());
                    }
                }

                if (build_info.config.node["strip-mimetype"])
                {
                    for (const auto& i :
                         build_info.config.node["strip-mimetype"])
                    {
                        mime_to_strip.emplace_back(i.as<std::string>());
                    }
                }

                if (std::find(mime_to_strip.begin(), mime_to_strip.end(),
                              mime_type) == mime_to_strip.end())
                {
                    continue;
                }

                try
                {
                    auto dbg_file_path = iter.path().string() + ".dbg";
                    // Copy debugging symbols to dbg directory
                    Executor("/bin/objcopy")
                        .arg("--only-keep-debug")
                        .arg(iter.path())
                        .arg(dbg_file_path)
                        .silent()
                        .execute();

                    std::string strip_args = "--strip-all";
                    if (iter.path().has_extension())
                    {
                        if (iter.path().extension() == ".a")
                        {
                            strip_args = "--strip-debug";
                        }
                        else
                        {
                            strip_args = "--strip-unneeded";
                        }
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
                }
                catch (const std::exception& exception)
                {
                    std::cerr << "failed to strip " << iter.path().string()
                              << " with mimetype " << mime_type << " because "
                              << exception.what() << std::endl;
                    continue;
                }
            }
        }
    }
};
