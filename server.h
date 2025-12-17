#pragma once

#include "fdm.h"
#include "config.h"
#include "reaper.h"
#include "wayland.h"

struct server;
struct server *server_init(struct config *conf, struct fdm *fdm,
                           struct reaper *reaper, struct wayland *wayl);
void server_destroy(struct server *server);

void server_global_theme_switch_to_1(struct server *server);
void server_global_theme_switch_to_2(struct server *server);
