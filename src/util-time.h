/*
 * Copyright © 2013-2019 Red Hat, Inc.
 *
 * Permission is hereby granted, free of charge, to any person obtaining a
 * copy of this software and associated documentation files (the "Software"),
 * to deal in the Software without restriction, including without limitation
 * the rights to use, copy, modify, merge, publish, distribute, sublicense,
 * and/or sell copies of the Software, and to permit persons to whom the
 * Software is furnished to do so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice (including the next
 * paragraph) shall be included in all copies or substantial portions of the
 * Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT.  IN NO EVENT SHALL
 * THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 * LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING
 * FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER
 * DEALINGS IN THE SOFTWARE.
 */

#pragma once

#include "config.h"

#include <assert.h>
#include <errno.h>
#include <linux/input.h>
#include <stdbool.h>
#include <stdint.h>
#include <time.h>
#include <unistd.h>

#include "util-macros.h"
#include "util-newtype.h"

DECLARE_NEWTYPE(usec, uint64_t);

static inline usec_t
usec_from_millis(uint32_t millis)
{
	return usec_from_uint64_t(millis * 1000);
}

static inline usec_t
usec_from_seconds(uint32_t secs)
{
	return usec_from_millis(secs * 1000);
}

static inline usec_t
usec_from_hours(uint32_t hours)
{
	return usec_from_seconds(hours * 3600);
}

static inline uint32_t
usec_to_millis(usec_t us)
{
	return usec_as_uint64_t(us) / 1000;
}

static inline uint32_t
usec_to_seconds(usec_t us)
{
	return usec_as_uint64_t(us) / 1000000;
}

static inline uint32_t
usec_to_minutes(usec_t us)
{
	return usec_to_seconds(us) / 60;
}

static inline uint32_t
usec_to_hours(usec_t us)
{
	return usec_to_minutes(us) / 60;
}

static inline usec_t
usec_add_millis(usec_t us, uint32_t millis)
{
	return usec_from_uint64_t(usec_as_uint64_t(us) + millis * 1000);
}

static inline usec_t
usec_delta(usec_t later, usec_t earlier)
{
	return usec_from_uint64_t(later.v - earlier.v);
}

static inline double
us2ms_f(usec_t us)
{
	return (double)usec_as_uint64_t(us) / 1000.0;
}

static inline usec_t
usec_from_timeval(const struct timeval *tv)
{
	return usec_from_uint64_t(tv->tv_sec * 1000000 + tv->tv_usec);
}

static inline usec_t
usec_from_timespec(const struct timespec *tp)
{
	return usec_from_uint64_t(tp->tv_sec * 1000000 + tp->tv_nsec / 1000);
}

static inline usec_t
usec_from_now(void)
{
	struct timespec ts = { 0, 0 };

	clock_gettime(CLOCK_MONOTONIC, &ts);

	return usec_from_timespec(&ts);
}

static inline struct timeval
usec_to_timeval(usec_t time)
{
	struct timeval tv;
	uint64_t time_us = usec_as_uint64_t(time);
	uint64_t one_sec_us = usec_as_uint64_t(usec_from_millis(1000));

	tv.tv_sec = time_us / one_sec_us;
	tv.tv_usec = time_us % one_sec_us;

	return tv;
}

static inline struct timespec
usec_to_timespec(usec_t time)
{
	struct timespec ts;
	uint64_t time_us = usec_as_uint64_t(time);
	uint64_t one_sec_us = usec_as_uint64_t(usec_from_millis(1000));

	ts.tv_sec = time_us / one_sec_us;
	ts.tv_nsec = (time_us % one_sec_us) * 1000;

	return ts;
}

static inline usec_t
usec_add(usec_t a, usec_t b)
{
	return usec_from_uint64_t(usec_as_uint64_t(a) + usec_as_uint64_t(b));
}

static inline usec_t
usec_sub(usec_t a, usec_t b)
{
	return usec_from_uint64_t(usec_as_uint64_t(a) - usec_as_uint64_t(b));
}

static inline usec_t
usec_div(usec_t a, uint64_t b)
{
	return usec_from_uint64_t(usec_as_uint64_t(a) / b);
}

static inline usec_t
usec_mul(usec_t a, double b)
{
	return usec_from_uint64_t(usec_as_uint64_t(a) * b);
}

static inline bool
usec_is_zero(usec_t a)
{
	return usec_as_uint64_t(a) == 0;
}

static inline void
msleep(unsigned int ms)
{
	usleep(ms * 1000);
}

static inline int
now_in_us(usec_t *us)
{
	struct timespec ts = { 0, 0 };

	if (clock_gettime(CLOCK_MONOTONIC, &ts) != 0) {
		*us = usec_from_uint64_t(0);
		return -errno;
	}

	*us = usec_from_timespec(&ts);

	return 0;
}

struct human_time {
	unsigned int value;
	const char *unit;
};

/**
 * Converts a time delta in µs to a human-readable time like "2h" or "4d"
 */
static inline struct human_time
to_human_time(usec_t us)
{
	struct human_time t;
	struct c {
		const char *unit;
		unsigned int change_from_previous;
		uint64_t limit;
	} conversion[] = {
		{ "us", 1, 5000 },  { "ms", 1000, 5000 }, { "s", 1000, 120 },
		{ "min", 60, 120 }, { "h", 60, 48 },      { "d", 24, ~0 },
	};
	uint64_t value = usec_as_uint64_t(us);

	ARRAY_FOR_EACH(conversion, c) {
		value = value / c->change_from_previous;
		if (value < c->limit) {
			t.unit = c->unit;
			t.value = value;
			return t;
		}
	}

	assert(!"We should never get here");
}
