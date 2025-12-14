#ifndef MEMORY_STREAM_H
#define MEMORY_STREAM_H

/**
 * Utility functions for memory streams.
 */

#include <stdio.h>
#include <stddef.h>
#include <stdbool.h>

struct memory_stream {
	FILE *fp;
	char *str;
	size_t str_len;
};

bool
memory_stream_open(struct memory_stream *m);

char *
memory_stream_close(struct memory_stream *m);

/**
 * A small cleanup helper that simply
 * calls free(memory_stream_close(m)) to avoid
 * any dangling string pointers in cleanup/error paths.
 */
void
memory_stream_cleanup(struct memory_stream *m);

#endif
