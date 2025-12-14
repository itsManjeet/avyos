#include "log.h"

void
_di_logger_va_add_failure(struct di_logger *logger, const char fmt[], va_list args)
{
	if (!logger->initialized) {
		if (ftell(logger->f) > 0) {
			fprintf(logger->f, "\n");
		}
		fprintf(logger->f, "%s:\n", logger->section);
		logger->initialized = true;
	}

	fprintf(logger->f, "  ");
	vfprintf(logger->f, fmt, args);
	fprintf(logger->f, "\n");
}

/**
 * Sometimes calling the functions that wrap _di_logger_va_add_failure() (e.g.
 * add_failure() in edid.c) is not possible, because we want to use a specific
 * logger (and not e.g. edid->logger). This allow us to log with a custom
 * logger. Avoid using this if not necessary.
 */
void
_di_logger_add_failure(struct di_logger *logger, const char fmt[], ...)
{
	va_list args;

	va_start(args, fmt);
	_di_logger_va_add_failure(logger, fmt, args);
	va_end(args);
}
