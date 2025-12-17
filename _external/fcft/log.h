#pragma once

#include <fcft/fcft.h>

void fcft_log_msg(enum fcft_log_class log_class, const char *module,
                  const char *file, int lineno,
                  const char *fmt, ...) __attribute__((format (printf, 5, 6)));

void fcft_log_errno(enum fcft_log_class log_class, const char *module,
                    const char *file, int lineno,
                    const char *fmt, ...) __attribute__((format (printf, 5, 6)));

void fcft_log_errno_provided(
    enum fcft_log_class log_class, const char *module,
    const char *file, int lineno, int _errno,
    const char *fmt, ...) __attribute__((format (printf, 6, 7)));

#define LOG_ERR(...)  \
    fcft_log_msg(FCFT_LOG_CLASS_ERROR, LOG_MODULE, __FILE__, __LINE__, __VA_ARGS__)
#define LOG_ERRNO(...) \
    fcft_log_errno(FCFT_LOG_CLASS_ERROR, LOG_MODULE, __FILE__, __LINE__, __VA_ARGS__)
#define LOG_ERRNO_P(_errno, ...)                                        \
    fcft_log_errno_provided(FCFT_LOG_CLASS_ERROR, LOG_MODULE, __FILE__, __LINE__, \
                            _errno, __VA_ARGS__)
#define LOG_WARN(...)  \
    fcft_log_msg(FCFT_LOG_CLASS_WARNING, LOG_MODULE, __FILE__, __LINE__, __VA_ARGS__)
#define LOG_INFO(...)  \
    fcft_log_msg(FCFT_LOG_CLASS_INFO, LOG_MODULE, __FILE__, __LINE__, __VA_ARGS__)

#if defined(LOG_ENABLE_DBG) && LOG_ENABLE_DBG
 #define LOG_DBG(...)  \
    fcft_log_msg(FCFT_LOG_CLASS_DEBUG, LOG_MODULE, __FILE__, __LINE__, __VA_ARGS__)
#else
 #define LOG_DBG(...)
#endif
