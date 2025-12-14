/*
 * Copyright © 2018 Rob Clark <robclark@freedesktop.org>
 * SPDX-License-Identifier: MIT
 *
 * Authors:
 *    Rob Clark <robclark@freedesktop.org>
 */

#ifndef FD5_RESOURCE_H_
#define FD5_RESOURCE_H_

#include "freedreno_resource.h"

uint32_t fd5_layout_resource(struct fd_resource *rsc, enum fd_layout_type type);

#endif /* FD5_RESOURCE_H_ */
