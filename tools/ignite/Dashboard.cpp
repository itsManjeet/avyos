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
#include "httplib.h"

#include <chrono>
#include <csignal>
#include <cstdio>
#include <cstdlib>
#include <cstring>
#include <fcntl.h>
#include <fstream>
#include <iostream>
#include <sstream>
#include <sys/stat.h>
#include <sys/wait.h>
#include <unistd.h>

namespace {

int64_t now_seconds() {
    return std::chrono::duration_cast<std::chrono::seconds>(
            std::chrono::system_clock::now().time_since_epoch())
            .count();
}

std::string json_escape(const std::string& value) {
    std::string out;
    out.reserve(value.size() + 2);
    out.push_back('"');
    for (char c : value) {
        switch (c) {
        case '"': out += "\\\""; break;
        case '\\': out += "\\\\"; break;
        case '\b': out += "\\b"; break;
        case '\f': out += "\\f"; break;
        case '\n': out += "\\n"; break;
        case '\r': out += "\\r"; break;
        case '\t': out += "\\t"; break;
        default:
            if (static_cast<unsigned char>(c) < 0x20) {
                char buf[8];
                std::snprintf(buf, sizeof(buf), "\\u%04x",
                        static_cast<unsigned char>(c));
                out += buf;
            } else {
                out.push_back(c);
            }
        }
    }
    out.push_back('"');
    return out;
}

std::string json_array(const std::vector<std::string>& values) {
    std::string out = "[";
    for (size_t i = 0; i < values.size(); ++i) {
        if (i) out += ",";
        out += json_escape(values[i]);
    }
    out += "]";
    return out;
}

std::string state_name(BuildJob::State state) {
    switch (state) {
    case BuildJob::State::Queued: return "queued";
    case BuildJob::State::Running: return "running";
    case BuildJob::State::Success: return "success";
    case BuildJob::State::Failed: return "failed";
    case BuildJob::State::Cancelled: return "cancelled";
    }
    return "unknown";
}

std::vector<std::string> parse_string_array(const std::string& body) {
    std::vector<std::string> result;
    size_t i = 0;
    auto skip_ws = [&]() {
        while (i < body.size() && std::isspace((unsigned char)body[i])) ++i;
    };

    skip_ws();
    size_t bracket = body.find('[', i);
    if (bracket == std::string::npos) return result;
    i = bracket + 1;
    while (i < body.size()) {
        skip_ws();
        if (i >= body.size()) break;
        if (body[i] == ']') break;
        if (body[i] != '"') { ++i; continue; }
        ++i;
        std::string value;
        while (i < body.size() && body[i] != '"') {
            if (body[i] == '\\' && i + 1 < body.size()) {
                char next = body[i + 1];
                switch (next) {
                case 'n': value.push_back('\n'); break;
                case 't': value.push_back('\t'); break;
                case 'r': value.push_back('\r'); break;
                case '"': value.push_back('"'); break;
                case '\\': value.push_back('\\'); break;
                case '/': value.push_back('/'); break;
                default: value.push_back(next); break;
                }
                i += 2;
            } else {
                value.push_back(body[i]);
                ++i;
            }
        }
        if (i < body.size()) ++i;
        result.push_back(value);
        skip_ws();
        if (i < body.size() && body[i] == ',') ++i;
    }
    return result;
}

std::string mime_for(const std::filesystem::path& path) {
    auto ext = path.extension().string();
    if (ext == ".html") return "text/html; charset=utf-8";
    if (ext == ".css") return "text/css; charset=utf-8";
    if (ext == ".js") return "application/javascript; charset=utf-8";
    if (ext == ".json") return "application/json";
    if (ext == ".svg") return "image/svg+xml";
    if (ext == ".ico") return "image/x-icon";
    return "application/octet-stream";
}

bool read_file(const std::filesystem::path& path, std::string& out) {
    std::ifstream in(path, std::ios::binary);
    if (!in) return false;
    std::stringstream ss;
    ss << in.rdbuf();
    out = ss.str();
    return true;
}

std::string resolve_exe_path() {
    char buf[4096];
    ssize_t n = readlink("/proc/self/exe", buf, sizeof(buf) - 1);
    if (n < 0) return "ignite";
    buf[n] = '\0';
    return std::string(buf);
}

} // namespace

Dashboard::Dashboard(Ignite& ignite, int port, std::string bind_host,
        std::filesystem::path project_path,
        std::filesystem::path cache_path, std::string arch,
        std::filesystem::path assets_path)
    : ignite(ignite), port(port), bind_host(std::move(bind_host)),
      project_path(std::move(project_path)),
      cache_path(std::move(cache_path)), arch(std::move(arch)),
      assets_path(std::move(assets_path)) {
    log_root = this->cache_path / "dashboard-logs";
    std::filesystem::create_directories(log_root);
}

std::string Dashboard::new_job_id() {
    auto t = now_seconds();
    auto n = ++job_counter;
    std::ostringstream oss;
    oss << t << "-" << n;
    return oss.str();
}

std::shared_ptr<BuildJob> Dashboard::find_job(const std::string& id) {
    std::lock_guard<std::mutex> lock(jobs_mutex);
    if (active && active->id == id) return active;
    for (auto const& job : queue) {
        if (job->id == id) return job;
    }
    for (auto const& job : history) {
        if (job->id == id) return job;
    }
    return nullptr;
}

std::string Dashboard::json_recipes() {
    std::string out = "[";
    bool first = true;
    for (auto const& [key, recipe] : ignite.get_pool()) {
        if (!first) out += ",";
        first = false;
        Recipe copy = recipe;
        std::string state = "unknown";
        std::string note;
        try {
            copy.cache = ignite.hash(copy);
            auto cached = std::filesystem::exists(ignite.cachefile(copy));
            auto workspace = ignite.workspace_available(copy);
            state = workspace ? "workspace" : (cached ? "cached" : "waiting");
        } catch (const std::exception& e) {
            note = e.what();
        }
        out += "{";
        out += "\"id\":" + json_escape(key);
        out += ",\"recipe_id\":" + json_escape(recipe.id);
        out += ",\"version\":" + json_escape(recipe.version);
        out += ",\"about\":" + json_escape(recipe.about);
        out += ",\"state\":" + json_escape(state);
        out += ",\"depends\":" + json_array(recipe.depends);
        if (!note.empty()) out += ",\"note\":" + json_escape(note);
        out += "}";
    }
    out += "]";
    return out;
}

std::string Dashboard::json_recipe(const std::string& id) {
    auto it = ignite.get_pool().find(id);
    if (it == ignite.get_pool().end()) return "null";
    Recipe copy = it->second;
    std::string state = "unknown";
    std::string note;
    try {
        copy.cache = ignite.hash(copy);
    } catch (const std::exception& e) {
        note = e.what();
    }
    try {
        copy.resolve(ignite.config);
    } catch (...) {
    }
    bool cached = false, workspace = false;
    if (!copy.cache.empty()) {
        cached = std::filesystem::exists(ignite.cachefile(copy));
        workspace = ignite.workspace_available(copy);
        state = workspace ? "workspace" : (cached ? "cached" : "waiting");
    }

    std::string out = "{";
    out += "\"id\":" + json_escape(id);
    out += ",\"recipe_id\":" + json_escape(copy.id);
    out += ",\"element_id\":" + json_escape(copy.element_id);
    out += ",\"version\":" + json_escape(copy.version);
    out += ",\"about\":" + json_escape(copy.about);
    out += ",\"cache\":" + json_escape(copy.cache);
    out += ",\"state\":" + json_escape(state);
    out += ",\"package_name\":" + json_escape(copy.package_name(copy.element_id));
    out += ",\"cache_file\":" + json_escape(ignite.cachefile(copy).string());
    out += ",\"depends\":" + json_array(copy.depends);
    out += ",\"build_time_depends\":" + json_array(copy.build_time_depends);
    out += ",\"sources\":" + json_array(copy.sources);
    out += ",\"backup\":" + json_array(copy.backup);
    out += ",\"integration\":" + json_escape(copy.integration);
    if (!note.empty()) out += ",\"note\":" + json_escape(note);
    out += "}";
    return out;
}

std::string Dashboard::json_status_summary() {
    size_t total = 0, cached = 0, workspace = 0, broken = 0;
    for (auto const& [key, recipe] : ignite.get_pool()) {
        Recipe copy = recipe;
        ++total;
        try {
            copy.cache = ignite.hash(copy);
        } catch (...) {
            ++broken;
            continue;
        }
        if (ignite.workspace_available(copy)) {
            ++workspace;
        } else if (std::filesystem::exists(ignite.cachefile(copy))) {
            ++cached;
        }
    }
    std::ostringstream oss;
    oss << "{\"total\":" << total << ",\"cached\":" << cached
        << ",\"workspace\":" << workspace
        << ",\"waiting\":" << (total - cached - workspace - broken)
        << ",\"broken\":" << broken
        << ",\"arch\":" << json_escape(arch)
        << ",\"project_path\":" << json_escape(project_path.string())
        << ",\"cache_path\":" << json_escape(cache_path.string())
        << "}";
    return oss.str();
}

std::string Dashboard::json_build(const std::shared_ptr<BuildJob>& job) {
    std::string out = "{";
    out += "\"id\":" + json_escape(job->id);
    out += ",\"state\":" + json_escape(state_name(job->state));
    out += ",\"recipes\":" + json_array(job->recipes);
    out += ",\"current_recipe\":" + json_escape(job->current_recipe);
    out += ",\"started_at\":" + std::to_string(job->started_at);
    out += ",\"finished_at\":" + std::to_string(job->finished_at);
    out += ",\"exit_code\":" + std::to_string(job->exit_code);
    out += ",\"log_path\":" + json_escape(job->log_path.string());
    out += "}";
    return out;
}

std::string Dashboard::json_builds_list() {
    std::lock_guard<std::mutex> lock(jobs_mutex);
    std::string out = "{\"active\":";
    out += active ? json_build(active) : "null";
    out += ",\"queue\":[";
    bool first = true;
    for (auto const& job : queue) {
        if (!first) out += ",";
        first = false;
        out += json_build(job);
    }
    out += "],\"history\":[";
    first = true;
    for (auto it = history.rbegin(); it != history.rend(); ++it) {
        if (!first) out += ",";
        first = false;
        out += json_build(*it);
    }
    out += "]}";
    return out;
}

void Dashboard::worker_loop() {
    while (!shutting_down) {
        std::shared_ptr<BuildJob> job;
        {
            std::unique_lock<std::mutex> lock(jobs_mutex);
            jobs_cv.wait(lock,
                    [&]() { return shutting_down || !queue.empty(); });
            if (shutting_down) return;
            job = queue.front();
            queue.pop_front();
            active = job;
            job->state = BuildJob::State::Running;
            job->started_at = now_seconds();
        }
        run_job(job);
        {
            std::lock_guard<std::mutex> lock(jobs_mutex);
            job->finished_at = now_seconds();
            history.push_back(job);
            if (history.size() > 100) {
                history.erase(history.begin(),
                        history.begin() + (history.size() - 100));
            }
            active.reset();
        }
    }
}

void Dashboard::run_job(const std::shared_ptr<BuildJob>& job) {
    auto exe = resolve_exe_path();
    int overall_status = 0;

    int log_fd = open(job->log_path.c_str(),
            O_WRONLY | O_CREAT | O_TRUNC | O_CLOEXEC, 0644);
    if (log_fd < 0) {
        job->state = BuildJob::State::Failed;
        job->exit_code = -1;
        return;
    }

    for (auto const& recipe_id : job->recipes) {
        {
            std::lock_guard<std::mutex> lock(jobs_mutex);
            job->current_recipe = recipe_id;
        }

        std::string banner =
                "==> building " + recipe_id + "\n";
        (void)!write(log_fd, banner.data(), banner.size());

        pid_t pid = fork();
        if (pid < 0) {
            overall_status = 1;
            break;
        }
        if (pid == 0) {
            dup2(log_fd, STDOUT_FILENO);
            dup2(log_fd, STDERR_FILENO);
            close(log_fd);
            setpgid(0, 0);
            std::vector<std::string> args{exe, "-project-path",
                    project_path.string(), "-cache-path", cache_path.string(),
                    "-arch", arch, "build", recipe_id};
            std::vector<char*> argv;
            argv.reserve(args.size() + 1);
            for (auto& a : args) argv.push_back(a.data());
            argv.push_back(nullptr);
            execv(exe.c_str(), argv.data());
            _exit(127);
        }

        {
            std::lock_guard<std::mutex> lock(jobs_mutex);
            job->pid = pid;
        }

        int status = 0;
        waitpid(pid, &status, 0);

        {
            std::lock_guard<std::mutex> lock(jobs_mutex);
            job->pid = -1;
        }

        if (WIFSIGNALED(status)) {
            std::string msg =
                    "==> terminated by signal " +
                    std::to_string(WTERMSIG(status)) + "\n";
            (void)!write(log_fd, msg.data(), msg.size());
            if (job->state == BuildJob::State::Cancelled) {
                break;
            }
            overall_status = 128 + WTERMSIG(status);
            break;
        }

        int exit_code = WEXITSTATUS(status);
        if (exit_code != 0) {
            overall_status = exit_code;
            std::string msg = "==> build failed for " + recipe_id +
                              " (exit " + std::to_string(exit_code) + ")\n";
            (void)!write(log_fd, msg.data(), msg.size());
            break;
        }
        std::string msg = "==> completed " + recipe_id + "\n";
        (void)!write(log_fd, msg.data(), msg.size());
    }

    close(log_fd);
    job->exit_code = overall_status;
    if (job->state != BuildJob::State::Cancelled) {
        job->state = overall_status == 0 ? BuildJob::State::Success
                                         : BuildJob::State::Failed;
    }
}

int Dashboard::run() {
    httplib::Server server;
    server.set_payload_max_length(4 * 1024 * 1024);

    server.set_default_headers({
            {"Access-Control-Allow-Origin", "*"},
    });

    server.Get("/api/status", [&](const httplib::Request&,
                                       httplib::Response& res) {
        res.set_content(json_status_summary(), "application/json");
    });

    server.Get("/api/recipes", [&](const httplib::Request&,
                                        httplib::Response& res) {
        res.set_content(json_recipes(), "application/json");
    });

    server.Get(R"(/api/recipes/(.+))", [&](const httplib::Request& req,
                                                httplib::Response& res) {
        auto id = req.matches[1].str();
        res.set_content(json_recipe(id), "application/json");
    });

    server.Get("/api/builds", [&](const httplib::Request&,
                                       httplib::Response& res) {
        res.set_content(json_builds_list(), "application/json");
    });

    server.Post("/api/builds", [&](const httplib::Request& req,
                                        httplib::Response& res) {
        auto recipes = parse_string_array(req.body);
        if (recipes.empty()) {
            res.status = 400;
            res.set_content(
                    "{\"error\":\"no recipes provided\"}", "application/json");
            return;
        }
        auto job = std::make_shared<BuildJob>();
        job->id = new_job_id();
        job->recipes = recipes;
        job->log_path = log_root / (job->id + ".log");
        {
            std::lock_guard<std::mutex> lock(jobs_mutex);
            queue.push_back(job);
        }
        jobs_cv.notify_all();
        res.set_content(json_build(job), "application/json");
    });

    server.Get(R"(/api/builds/([^/]+))", [&](const httplib::Request& req,
                                                  httplib::Response& res) {
        auto job = find_job(req.matches[1].str());
        if (!job) {
            res.status = 404;
            res.set_content("{\"error\":\"not found\"}", "application/json");
            return;
        }
        res.set_content(json_build(job), "application/json");
    });

    server.Post(R"(/api/builds/([^/]+)/cancel)",
            [&](const httplib::Request& req, httplib::Response& res) {
                auto job = find_job(req.matches[1].str());
                if (!job) {
                    res.status = 404;
                    res.set_content(
                            "{\"error\":\"not found\"}", "application/json");
                    return;
                }
                {
                    std::lock_guard<std::mutex> lock(jobs_mutex);
                    if (job->state == BuildJob::State::Queued) {
                        job->state = BuildJob::State::Cancelled;
                        job->finished_at = now_seconds();
                        for (auto it = queue.begin(); it != queue.end(); ++it) {
                            if ((*it)->id == job->id) {
                                queue.erase(it);
                                break;
                            }
                        }
                        history.push_back(job);
                    } else if (job->state == BuildJob::State::Running) {
                        job->state = BuildJob::State::Cancelled;
                        if (job->pid > 0) {
                            kill(-job->pid, SIGTERM);
                        }
                    }
                }
                res.set_content(json_build(job), "application/json");
            });

    server.Get(R"(/api/builds/([^/]+)/logs)",
            [&](const httplib::Request& req, httplib::Response& res) {
                auto job = find_job(req.matches[1].str());
                if (!job) {
                    res.status = 404;
                    res.set_content(
                            "{\"error\":\"not found\"}", "application/json");
                    return;
                }
                res.set_header("Cache-Control", "no-cache");
                res.set_header("X-Accel-Buffering", "no");
                auto log_path = job->log_path;
                auto job_weak = std::weak_ptr<BuildJob>(job);
                res.set_chunked_content_provider(
                        "text/event-stream",
                        [log_path, job_weak](
                                size_t, httplib::DataSink& sink) {
                            int fd = open(log_path.c_str(),
                                    O_RDONLY | O_CLOEXEC);
                            auto start = std::chrono::steady_clock::now();
                            while (fd < 0) {
                                if (std::chrono::steady_clock::now() - start >
                                        std::chrono::seconds(5)) {
                                    sink.done();
                                    return true;
                                }
                                std::this_thread::sleep_for(
                                        std::chrono::milliseconds(200));
                                fd = open(log_path.c_str(),
                                        O_RDONLY | O_CLOEXEC);
                            }
                            std::string pending;
                            char buf[4096];
                            while (true) {
                                ssize_t n = read(fd, buf, sizeof(buf));
                                if (n > 0) {
                                    pending.append(buf, n);
                                    size_t pos;
                                    while ((pos = pending.find('\n')) !=
                                            std::string::npos) {
                                        std::string line =
                                                pending.substr(0, pos);
                                        pending.erase(0, pos + 1);
                                        std::string out = "data: " +
                                                          json_escape(line) +
                                                          "\n\n";
                                        if (!sink.write(out.data(),
                                                    out.size())) {
                                            close(fd);
                                            return true;
                                        }
                                    }
                                    continue;
                                }
                                auto job = job_weak.lock();
                                bool done = !job ||
                                            (job->state !=
                                                            BuildJob::State::
                                                                    Running &&
                                                    job->state !=
                                                            BuildJob::State::
                                                                    Queued);
                                if (done) {
                                    if (!pending.empty()) {
                                        std::string out = "data: " +
                                                          json_escape(
                                                                  pending) +
                                                          "\n\n";
                                        sink.write(out.data(), out.size());
                                    }
                                    std::string final_event;
                                    if (job) {
                                        final_event = "event: end\ndata: " +
                                                      state_name(job->state) +
                                                      "\n\n";
                                    } else {
                                        final_event =
                                                "event: end\ndata: gone\n\n";
                                    }
                                    sink.write(final_event.data(),
                                            final_event.size());
                                    close(fd);
                                    sink.done();
                                    return true;
                                }
                                std::this_thread::sleep_for(
                                        std::chrono::milliseconds(400));
                            }
                        });
            });

    server.Post("/api/fetch", [&](const httplib::Request& req,
                                       httplib::Response& res) {
        auto recipes = parse_string_array(req.body);
        try {
            ignite.fetch_sources(recipes, false);
            res.set_content("{\"ok\":true}", "application/json");
        } catch (const std::exception& e) {
            res.status = 500;
            res.set_content(
                    std::string("{\"error\":") + json_escape(e.what()) + "}",
                    "application/json");
        }
    });

    server.set_mount_point("/", assets_path.string());

    server.Get("/", [&](const httplib::Request&, httplib::Response& res) {
        std::string content;
        if (read_file(assets_path / "index.html", content)) {
            res.set_content(content, "text/html; charset=utf-8");
        } else {
            res.status = 500;
            res.set_content(
                    "dashboard assets missing at " + assets_path.string(),
                    "text/plain");
        }
    });

    worker = std::thread(&Dashboard::worker_loop, this);

    std::cout << "ignite dashboard listening on http://" << bind_host << ":"
              << port << std::endl;
    std::cout << "serving assets from " << assets_path << std::endl;

    bool ok = server.listen(bind_host.c_str(), port);

    shutting_down = true;
    jobs_cv.notify_all();
    if (worker.joinable()) worker.join();

    return ok ? 0 : 1;
}
