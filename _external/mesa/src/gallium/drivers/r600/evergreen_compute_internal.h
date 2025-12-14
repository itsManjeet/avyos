/*
 * Authors:
 *      Adam Rak <adam.rak@streamnovation.com>
 * SPDX-License-Identifier: MIT
 */

#ifndef EVERGREEN_COMPUTE_INTERNAL_H
#define EVERGREEN_COMPUTE_INTERNAL_H

#include "ac_binary.h"

struct r600_pipe_compute {
	struct r600_context *ctx;

	/* tgsi selector */
	struct r600_pipe_shader_selector *sel;

	unsigned local_size;
};

struct r600_resource* r600_compute_buffer_alloc_vram(struct r600_screen *screen, unsigned size);

#endif
