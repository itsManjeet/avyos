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

#include "Ignite.h"
#include <atomic>
#include <condition_variable>
#include <deque>
#include <filesystem>
#include <memory>
#include <mutex>
#include <string>
#include <thread>
#include <vector>

struct BuildJob {
    enum class State { Queued, Running, Success, Failed, Cancelled };

    std::string id;
    std::vector<std::string> recipes;
    std::string current_recipe;
    State state{State::Queued};
    int64_t started_at{0};
    int64_t finished_at{0};
    int exit_code{0};
    std::filesystem::path log_path;
    pid_t pid{-1};
};

class Dashboard {
    Ignite& ignite;
    int port;
    std::string bind_host;
    std::filesystem::path project_path;
    std::filesystem::path cache_path;
    std::string arch;
    std::filesystem::path assets_path;
    std::filesystem::path log_root;

    std::mutex jobs_mutex;
    std::condition_variable jobs_cv;
    std::deque<std::shared_ptr<BuildJob>> queue;
    std::vector<std::shared_ptr<BuildJob>> history;
    std::shared_ptr<BuildJob> active;
    std::atomic<bool> shutting_down{false};
    std::atomic<uint64_t> job_counter{0};
    std::thread worker;

public:
    Dashboard(Ignite& ignite, int port, std::string bind_host,
            std::filesystem::path project_path,
            std::filesystem::path cache_path, std::string arch,
            std::filesystem::path assets_path);

    int run();

private:
    void worker_loop();
    void run_job(const std::shared_ptr<BuildJob>& job);
    std::shared_ptr<BuildJob> find_job(const std::string& id);
    std::string new_job_id();

    std::string json_recipes();
    std::string json_recipe(const std::string& id);
    std::string json_status_summary();
    std::string json_builds_list();
    std::string json_build(const std::shared_ptr<BuildJob>& job);
    std::string json_config();
};
