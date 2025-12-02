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

#include "json.hpp"

#include <yaml-cpp/node/parse.h>

#include <algorithm>
#include <cstring>
#include <expected>
#include <filesystem>
#include <functional>
#include <iostream>
#include <print>
#include <regex>
#include <vector>

import ignite;

using json = nlohmann::json;

std::filesystem::path project_path = std::filesystem::current_path();
std::filesystem::path cache_path;
std::string arch = "x86_64";

int help(Ignite* ignite, const std::vector<std::string>& args)
{
    std::cout << R"(Usage: ignite <options> <command> <args...>
Commands:
  build <recipes...>        Build artifact of specified recipes
  status <recipe>           Print if artifact is cached or need to build
  pull <recipe>             Pull artifact cache from artifact-url:
  cache-path <recipe>       Print the cache path of recipe
  update                    Check for package updates
  checkout <recipe> <path>  Checkout artifact at <path>

Options:
  -project-path <path>      Specify project path
  -cache-path <path>        Specify cache path
  -arch <arch>              Specify target device architecture (default: x86_64)
)" << std::endl;

    return 1;
}

int pull(Ignite* ignite, const std::vector<std::string>& args)
{
    std::vector<Ignite::State> states;
    ignite->resolve(args, states);
    const auto artifact_url = ignite->config.get<std::string>(
        "artifact-url", "https://repo.avyos.dev");
    std::filesystem::create_directories(cache_path);

    for (auto& [id, recipe, cached] : states)
    {
        if (!cached)
        {
            try
            {
                recipe.resolve(ignite->config);
            }
            catch (const std::exception& exception)
            {
                std::cerr << "ERROR: " << exception.what() << " " << id
                          << std::endl;
                return 1;
            }

            auto hash = ignite->hash(recipe);
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
            if (status != 0)
            {
                std::cerr << "Error: " << status << std::endl;
                return 1;
            }
        }
    }
    return 0;
}

int get_cache_path(Ignite* ignite, const std::vector<std::string>& args)
{
    if (args.size() != 1)
    {
        std::cerr << "require exactly one argument" << std::endl;
        return 1;
    }

    auto recipe = ignite->get_pool().find(args[0]);
    if (recipe == ignite->get_pool().end())
    {
        std::cerr << "no recipe found with id '" << args[0] << "'" << std::endl;
        return 1;
    }

    recipe->second.cache = ignite->hash(recipe->second);
    std::cout << ignite->cachefile(recipe->second) << std::endl;
    return 0;
}

int checkout(Ignite* ignite, const std::vector<std::string>& args)
{
    if (args.size() != 2)
    {
        std::cerr << "require exactly one argument" << std::endl;
        return 1;
    }

    auto recipe = ignite->get_pool().find(args[0]);
    if (recipe == ignite->get_pool().end())
    {
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

int build(Ignite* ignite, const std::vector<std::string>& args)
{
    std::vector<Ignite::State> states;
    ignite->resolve(args, states);
    for (auto& [id, recipe, cached] : states)
    {
        if (!cached)
        {
            try
            {
                recipe.resolve(ignite->config);
                std::cout << "building " << id << std::endl;
                ignite->build(recipe);
            }
            catch (const std::exception& exception)
            {
                std::cerr << "ERROR: " << exception.what() << " " << id
                          << std::endl;
                return 1;
            }
        }
    }
    return 0;
}

int status(Ignite* ignite, const std::vector<std::string>& args)
{
    std::vector<Ignite::State> states;
    ignite->resolve(args, states);
    int total_cached = 0;
    for (const auto& [id, recipe, cached] : states)
    {
        std::cout << "  " << (cached ? "CACHED " : "WAITING") << "  " << id
                  << std::endl;
        if (cached)
            ++total_cached;
    }

    std::cout << '\n'
              << "  TOTAL COMPONENTS : " << states.size() << '\n'
              << "  TOTAL CACHED     : " << total_cached << '\n'
              << "  NEED TO BUILD    : " << states.size() - total_cached
              << '\n';
    return 0;
}

auto check_update(Recipe* recipe)
    -> std::expected<std::vector<std::string>, std::string>
{
    if (recipe->sources.empty())
        return std::unexpected("source file is empty");

    auto url = recipe->sources[0];
    url = url.substr(0, url.find_last_of('/'));
    auto name = recipe->id;

    auto contains =
        [](const std::string& url, const std::vector<std::string>& s) -> bool {
        for (const auto a : s)
        {
            if (url.find(a) != std::string::npos)
            {
                return true;
            }
        }
        return false;
    };

    auto update_config = YAML::LoadFile("updates.yml");
    if (const auto aliases = update_config["aliases"])
    {
        if (aliases[name])
        {
            name = aliases[name].as<std::string>();
        }
    }

    std::function<std::vector<std::string>(const std::string& content,
                                           const std::string& filename)>
        filter = [](const std::string& content,
                    const std::string& filename) -> std::vector<std::string> {
        std::vector<std::string> versions;
        std::regex re(filename + "[-_][0-9a-z.]+\\.tar\\.[bgx]z2?");
        for (auto i = std::sregex_iterator(content.begin(), content.end(), re);
             i != std::sregex_iterator(); i++)
        {
            const auto match = i->str();
            auto version = std::regex_replace(
                match, std::regex("^" + filename + "[-_]"), "");

            version =
                std::regex_replace(version, std::regex("\\.tar\\..*$"), "");

            if (std::find(versions.begin(), versions.end(), version) ==
                versions.end())
            {
                versions.push_back(version);
            }
        }
        return versions;
    };

    auto get_part =
        [](const std::string& url, int start, int end) -> std::string {
        std::istringstream stream(url);
        std::string part;
        std::vector<std::string> tokens;

        while (std::getline(stream, part, '/'))
        {
            tokens.push_back(part);
        }

        if (start > 0 && end >= start && end <= static_cast<int>(tokens.size()))
        {
            std::ostringstream result;
            for (int i = start - 1; i < end; ++i)
            {
                if (i > start - 1)
                {
                    result << '/';
                }
                result << tokens[i];
            }
            return result.str();
        }

        return "";
    };

    if (update_config["url"] && update_config["url"][name])
    {
        url = update_config["url"][name].as<std::string>();
    }
    else
    {
        if (contains(url, {"github.com"}))
        {
            url = "https://github.com/" + get_part(url, 4, 5) + "/tags";
        }
        else if (contains(url, {"gitlab.com"}))
        {
            url = "https://gitlab.com/" + get_part(url, 4, 5) + "/tags";
        }
        else if (contains(url, {"downloads.sourceforge.net"}))
        {
            url = "https://sourceforge.net/projects/" + get_part(url, 4, 4) +
                  "/rss?limit=200";
        }
        else if (contains(url, {"sourceforge.net"}))
        {
            url = "https://sourceforge.net/projects/" + get_part(url, 5, 5) +
                  "/rss?limit=200";
        }
        else if (contains(url, {"ftp.gnome.org", "download.gnome.org"}))
        {
            url = "https://download.gnome.org/sources/" + get_part(url, 5, 5) +
                  "/cache.json";
            filter =
                [](const std::string& content,
                   const std::string& filename) -> std::vector<std::string> {
                std::vector<std::string> versions;
                auto doc = json::parse(content);
                for (const auto& [key, value] : doc[1][filename].items())
                {
                    versions.emplace_back(key);
                }
                return versions;
            };
        }
        else if (contains(url, {"archive.xfce.org"}))
        {
            url = "https://archive.xfce.org/src/" + get_part(url, 5, 5) + "/" +
                  recipe->id + "/";
        }
        else if (contains(url, {"python.org", "pypi.org", "pythonhosted.org",
                                "pypi.io"}))
        {
            url = "https://pypi.org/simple/" + name;
        }
        else if (contains(url, {"rubygems.org"}))
        {
            url = "https://rubygems.org/gems/" + name;
        }
        else if (contains(url, {"kde.org/stable"}))
        {}
    }

    std::println("checking url {}", url);
    auto [status, output] = Executor("curl").arg("-Lsk").arg(url).output();
    if (status != 0)
    {
        return std::unexpected(output);
    }

    auto versions = filter(output, name);

    return versions;
}

int update(Ignite* ignite, const std::vector<std::string>& args)
{
    std::vector<Ignite::State> states;
    ignite->resolve(args, states);

    auto update_config = YAML::LoadFile("updates.yml");
    auto aliases = update_config["aliases"];
    auto skip = update_config["skip"];

    auto recipe = std::get<1>(states[states.size() - 1]);

    recipe.resolve(ignite->config);
    auto name = recipe.id;
    const auto version = recipe.version;

    if (aliases && aliases[name])
    {
        name = aliases[name].as<std::string>();
    }

    if (skip && skip[name])
    {
        std::println("skipping {}", name);
    }

    auto result = check_update(&recipe);
    if (!result)
    {
        std::println("failed to check updates for {}: {}", name,
                     result.error());
        return 0;
    }

    std::println("{} available versions for {}", result.value().size(), name);
    for (const auto& version : *result)
    {
        std::println("- {}{}", version,
                     version == recipe.version ? " <- in use" : "");
    }
    return 0;
}


int main(int argc, char** argv)
{
    std::function<int(Ignite*, std::vector<std::string>)> function;
    std::vector<std::string> args;

    for (int i = 1; i < argc; ++i)
    {
        if (argv[i][0] == '-')
        {
            if (std::strcmp(argv[i], "-project-path") == 0)
            {
                project_path = argv[++i];
            }
            else if (std::strcmp(argv[i], "-cache-path") == 0)
            {
                cache_path = argv[++i];
            }
            else if (std::strcmp(argv[i], "-arch") == 0)
            {
                arch = argv[++i];
            }
            else
            {
                std::cerr << "Unknown option: " << argv[i] << std::endl;
                return 1;
            }
        }
        else if (function == nullptr)
        {
            if (std::strcmp(argv[i], "build") == 0)
            {
                function = build;
            }
            else if (std::strcmp(argv[i], "help") == 0)
            {
                function = help;
            }
            else if (std::strcmp(argv[i], "status") == 0)
            {
                function = status;
            }
            else if (std::strcmp(argv[i], "pull") == 0)
            {
                function = pull;
            }
            else if (std::strcmp(argv[i], "cache-path") == 0)
            {
                function = get_cache_path;
            }
            else if (std::strcmp(argv[i], "checkout") == 0)
            {
                function = checkout;
            }
            else if (std::strcmp(argv[i], "update") == 0)
            {
                function = update;
            }
            else
            {
                std::cerr << "Unknown option: " << argv[i] << std::endl;
                return 1;
            }
        }
        else
        {
            args.emplace_back(argv[i]);
        }
    }

    if (cache_path.empty())
    {
        cache_path = project_path / "build" / arch;
    }

    try
    {
        Configuration configuration;
        Ignite ignite(configuration, project_path, cache_path, arch);

        if (function == nullptr)
        {
            return help(&ignite, args);
        }

        ignite.load();

        return function(&ignite, args);
    }
    catch (const std::exception& exception)
    {
        std::cerr << "ERROR: " << exception.what() << std::endl;
        return 1;
    }
}
