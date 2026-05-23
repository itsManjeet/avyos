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

#include "Dashboard.h"
#include "Executor.h"
#include "Ignite.h"
#include <cstring>
#include <functional>
#include <iostream>
#include <stdexcept>
#include <unistd.h>

std::filesystem::path project_path = std::filesystem::current_path();
std::filesystem::path cache_path;
std::string arch = "x86_64";
bool force = false;
int dashboard_port = 8080;
std::string dashboard_host = "127.0.0.1";
std::filesystem::path dashboard_assets;

int help(Ignite* ignite, const std::vector<std::string>& args) {
    std::cout << R"(Usage: ignite <options> <command> <args...>
Commands:
  build <recipes...>        Build artifact of specified recipes
  status <recipe>           Print if artifact is cached or need to build
  pull <recipe>             Pull artifact cache from artifact-url:
  cache-path <recipe>       Print the cache path of recipe
  checkout <recipe> <path>  Checkout artifact at <path>
  fetch [recipes...]        Fetch sources and write checksum.lock
  workspace <recipe>        Create an editable source workspace
  workspace-finish <recipe> Export quilt patches and close the workspace
  dashboard                 Start a web dashboard to trigger and monitor builds

Options:
  -project-path <path>      Specify project path
  -cache-path <path>        Specify cache path
  -arch <arch>              Specify target device architecture (default: x86_64)
  -force                    Force refetch/update for supported commands
  -port <port>              Dashboard listen port (default: 8080)
  -host <host>              Dashboard bind host (default: 127.0.0.1)
  -assets <path>            Path to dashboard static assets
)" << std::endl;

    return 1;
}

Recipe find_recipe(Ignite* ignite, const std::string& component) {
    std::vector<std::string> candidates{component};
    if (!component.ends_with(".yml")) {
        candidates.push_back(component + ".yml");
    }
    if (!component.starts_with("components/")) {
        candidates.push_back("components/" + component);
        if (!component.ends_with(".yml")) {
            candidates.push_back("components/" + component + ".yml");
        }
    }

    for (auto const& candidate : candidates) {
        auto recipe = ignite->get_pool().find(candidate);
        if (recipe != ignite->get_pool().end()) { return recipe->second; }
    }

    auto found = ignite->get_pool().end();
    for (auto recipe = ignite->get_pool().begin();
            recipe != ignite->get_pool().end(); ++recipe) {
        if (recipe->second.id != component) { continue; }
        if (found != ignite->get_pool().end()) {
            throw std::runtime_error(
                    "multiple recipes found with id '" + component + "'");
        }
        found = recipe;
    }
    if (found != ignite->get_pool().end()) { return found->second; }

    throw std::runtime_error("no recipe found with id '" + component + "'");
}

Recipe find_element_recipe(Ignite* ignite, const std::string& component) {
    auto recipe = ignite->get_pool().find(component);
    if (recipe != ignite->get_pool().end()) { return recipe->second; }

    throw std::runtime_error(
            "no recipe found with element id '" + component + "'");
}

int pull(Ignite* ignite, const std::vector<std::string>& args) {
    std::vector<Ignite::State> states;
    ignite->resolve(args, states);
    auto const artifact_url = ignite->config.get<std::string>(
            "artifact-url", "https://repo.avyos.dev");

    for (auto& [id, recipe, cached] : states) {
        if (ignite->workspace_available(recipe)) {
            std::cout << "SKIP workspace active for " << id << std::endl;
            continue;
        }
        if (!cached) {
            recipe.resolve(ignite->config);
            auto server_url = artifact_url + "/cache/" +
                              recipe.package_name(recipe.element_id);
            auto cache_file_path = ignite->cachefile(recipe);
            std::cout << "GET " << server_url << std::endl;
            int status = Executor("/bin/curl")
                                 .arg("-C")
                                 .arg("-")
                                 .arg(server_url)
                                 .arg("-o")
                                 .arg(cache_file_path)
                                 .run();
            if (status != 0) {
                std::cerr << "Error: " << status << std::endl;
                return 1;
            }
        }
    }
    return 0;
}

int cachepath(Ignite* ignite, const std::vector<std::string>& args) {
    if (args.size() != 1) {
        std::cerr << "require exactly one argument" << std::endl;
        return 1;
    }

    auto recipe = find_recipe(ignite, args[0]);
    recipe.cache = ignite->hash(recipe);
    std::cout << ignite->cachefile(recipe) << std::endl;
    return 0;
}

int checkout(Ignite* ignite, const std::vector<std::string>& args) {
    if (args.size() != 2) {
        std::cerr << "require exactly two arguments: <recipe> <path>" << std::endl;
        return 1;
    }

    auto recipe = ignite->get_pool().find(args[0]);
    if (recipe == ignite->get_pool().end()) {
        std::cerr << "no recipe found with id '" << args[0] << "'" << std::endl;
        return 1;
    }

    recipe->second.cache = ignite->hash(recipe->second);
    std::filesystem::create_directories(args[1]);

    return Executor("/bin/tar")
            .arg("-xf")
            .arg(ignite->cachefile(recipe->second))
            .arg("-C")
            .arg(args[1])
            .run();
}

int fetch(Ignite* ignite, const std::vector<std::string>& args) {
    ignite->fetch_sources(args, force);
    return 0;
}

int build(Ignite* ignite, const std::vector<std::string>& args) {
    std::vector<Ignite::State> states;
    ignite->resolve(args, states);
    for (auto& [id, recipe, cached] : states) {
        if (!cached) {
            recipe.resolve(ignite->config);
            std::cout << "building " << id << std::endl;
            ignite->build(recipe);
        }
    }
    return 0;
}

int status(Ignite* ignite, const std::vector<std::string>& args) {
    std::vector<Ignite::State> states;
    ignite->resolve(args, states);
    int total_cached = 0;
    for (auto const& [id, recipe, cached] : states) {
        auto state = ignite->workspace_available(recipe) ? "WORKSPACE"
                         : cached                         ? "CACHED   "
                                                          : "WAITING  ";
        std::cout << "  " << state << "  " << id << std::endl;
        if (cached) ++total_cached;
    }

    std::cout << '\n'
              << "  TOTAL COMPONENTS : " << states.size() << '\n'
              << "  TOTAL CACHED     : " << total_cached << '\n'
              << "  NEED TO BUILD    : " << states.size() - total_cached
              << '\n';
    return 0;
}

int workspace(Ignite* ignite, const std::vector<std::string>& args) {
    if (args.size() != 1) {
        std::cerr << "require exactly one argument: <recipe>" << std::endl;
        return 1;
    }

    auto recipe = find_element_recipe(ignite, args[0]);
    recipe.cache = ignite->hash(recipe);
    recipe.resolve(ignite->config);
    ignite->workspace_init(recipe);
    return 0;
}

int dashboard(Ignite* ignite, const std::vector<std::string>& args) {
    (void)args;
    std::filesystem::path assets = dashboard_assets;
    if (assets.empty()) {
        char exe[4096];
        ssize_t n = readlink("/proc/self/exe", exe, sizeof(exe) - 1);
        if (n > 0) {
            exe[n] = '\0';
            auto exe_dir = std::filesystem::path(exe).parent_path();
            std::vector<std::filesystem::path> candidates{
                    exe_dir / "dashboard",
                    exe_dir / ".." / "share" / "ignite" / "dashboard",
                    project_path / "tools" / "ignite" / "dashboard",
            };
            for (auto const& c : candidates) {
                if (std::filesystem::exists(c / "index.html")) {
                    assets = std::filesystem::canonical(c);
                    break;
                }
            }
        }
        if (assets.empty()) {
            assets = project_path / "tools" / "ignite" / "dashboard";
        }
    }

    Dashboard server(*ignite, dashboard_port, dashboard_host, project_path,
            cache_path, arch, assets);
    return server.run();
}

int workspace_finish(Ignite* ignite, const std::vector<std::string>& args) {
    if (args.size() != 1) {
        std::cerr << "require exactly one argument: <recipe>" << std::endl;
        return 1;
    }

    auto recipe = find_element_recipe(ignite, args[0]);
    recipe.cache = ignite->hash(recipe);
    recipe.resolve(ignite->config);
    ignite->workspace_finish(recipe);
    return 0;
}

int main(int argc, char** argv) {

    std::function<int(Ignite*, std::vector<std::string>)> function;
    std::vector<std::string> args;

    for (int i = 1; i < argc; ++i) {
        if (argv[i][0] == '-') {
            auto require_arg = [&](const char* opt) -> bool {
                if (i + 1 >= argc) {
                    std::cerr << "Option " << opt << " requires an argument"
                              << std::endl;
                    return false;
                }
                return true;
            };
            if (std::strcmp(argv[i], "-project-path") == 0) {
                if (!require_arg("-project-path")) return 1;
                project_path = argv[++i];
            } else if (std::strcmp(argv[i], "-cache-path") == 0) {
                if (!require_arg("-cache-path")) return 1;
                cache_path = argv[++i];
            } else if (std::strcmp(argv[i], "-arch") == 0) {
                if (!require_arg("-arch")) return 1;
                arch = argv[++i];
            } else if (std::strcmp(argv[i], "-force") == 0) {
                force = true;
            } else if (std::strcmp(argv[i], "-port") == 0) {
                if (!require_arg("-port")) return 1;
                dashboard_port = std::stoi(argv[++i]);
            } else if (std::strcmp(argv[i], "-host") == 0) {
                if (!require_arg("-host")) return 1;
                dashboard_host = argv[++i];
            } else if (std::strcmp(argv[i], "-assets") == 0) {
                if (!require_arg("-assets")) return 1;
                dashboard_assets = argv[++i];
            } else {
                std::cerr << "Unknown option: " << argv[i] << std::endl;
                return 1;
            }
        } else if (!function) {
            if (std::strcmp(argv[i], "build") == 0) {
                function = build;
            } else if (std::strcmp(argv[i], "help") == 0) {
                function = help;
            } else if (std::strcmp(argv[i], "status") == 0) {
                function = status;
            } else if (std::strcmp(argv[i], "pull") == 0) {
                function = pull;
            } else if (std::strcmp(argv[i], "cache-path") == 0) {
                function = cachepath;
            } else if (std::strcmp(argv[i], "checkout") == 0) {
                function = checkout;
            } else if (std::strcmp(argv[i], "fetch") == 0) {
                function = fetch;
            } else if (std::strcmp(argv[i], "workspace") == 0) {
                function = workspace;
            } else if (std::strcmp(argv[i], "workspace-finish") == 0) {
                function = workspace_finish;
            } else if (std::strcmp(argv[i], "dashboard") == 0) {
                function = dashboard;
            } else {
                std::cerr << "Unknown option: " << argv[i] << std::endl;
                return 1;
            }
        } else {
            args.emplace_back(argv[i]);
        }
    }

    if (cache_path.empty()) { cache_path = project_path / "build" / arch; }

    try {
        Configuration configuration;
        Ignite ignite(configuration, project_path, cache_path, arch);

        if (!function) { return help(&ignite, args); }

        ignite.load();

        return function(&ignite, args);
    } catch (const std::exception& exception) {
        std::cerr << "ERROR: " << exception.what() << std::endl;
        return 1;
    }
}
