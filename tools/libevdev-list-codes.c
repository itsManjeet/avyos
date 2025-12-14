// SPDX-License-Identifier: MIT
/*
 * Copyright © 2021 Red Hat, Inc.
 */

/* Lists all event types and codes currently known by libevdev. */

#include "config.h"

#include <stdio.h>
#include <linux/input.h>
#include "libevdev/libevdev.h"

static void
list_event_codes(unsigned int type, unsigned int max)
{
	const char *typestr = libevdev_event_type_get_name(type);

	if (!typestr)
		return;

	printf("- %s:\n", typestr);

	for (unsigned int code = 0; code <= max; code++) {
		const char *str = libevdev_event_code_get_name(type, code);

		if (!str)
			continue;

		printf("    %d: %s\n", code, str);
	}
}

int
main (int argc, char **argv)
{
	printf("codes:\n");
	for (unsigned int type = 0; type <= EV_MAX; type++) {
		int max = libevdev_event_type_get_max(type);
		if (max == -1)
			continue;

		list_event_codes(type, (unsigned int)max);
	}

	return 0;
}
