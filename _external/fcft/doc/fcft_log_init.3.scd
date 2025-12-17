fcft_log_init(3) "3.3.2" "fcft"

# NAME

fcft_log_init - configure logging in fcft

# SYNOPSIS

*\#include <fcft/fcft.h>*

*void fcft_log_init(
	enum fcft_log_colorize* _colorize_*, bool* _do\_syslog_*,
	enum fcft_log_class* _log\_level_*);*

# DESCRIPTION

By default, fcft does not log anything; use *fcft_log_init*() to
enable, and configure logging in fcft.

*fcft_log_init*() is typically called early, at program startup.

_colorize_ is one of:

```
enum fcft_log_colorize {
    FCFT_LOG_COLORIZE_NEVER,
    FCFT_LOG_COLORIZE_ALWAYS,
    FCFT_LOG_COLORIZE_AUTO
};
```

*FCFT_LOG_COLORIZE_AUTO* enables colors if stderr is a TTY.

if _do\_syslog_ is set, fcft will log to syslog, in addition to the
regular logging on stderr.

Note that fcft does not call *openlog*(), even if _do\_syslog_ is
set. Do this from the calling application. Before, or after calling
*fcft_log_init*() does not matter, as long as it is done before
calling any other fcft API functions.


Finally, _log\_level_ configures which types of log messages that will be shown.
```

enum fcft_log_class {
    FCFT_LOG_CLASS_NONE,
    FCFT_LOG_CLASS_ERROR,
    FCFT_LOG_CLASS_WARNING,
    FCFT_LOG_CLASS_INFO,
    FCFT_LOG_CLASS_DEBUG
};
```

Note that this is not a bitmask; setting *FCFT_LOG_CLASS_INFO*, also
enables *FCFT_LOG_CLASS_WARNING* and *FCFT_LOG_CLASS_ERROR*.
