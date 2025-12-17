fcft_init(3) "3.3.2" "fcft"

# NAME

fcft_init - initialize the library

# SYNOPSIS

*\#include <fcft/fcft.h>*

*bool fcft_init(
	enum fcft_log_colorize* _colorize_*, bool* _do\_syslog_*,
	enum fcft_log_class* _log\_level_*);*

# DESCRIPTION

*fcft_init*() initializes the fcft library, and must be called before
instantiating any fonts.

Since it also initializes logging, you may also want to ensure it is
called before any other functions affecting the library state. For
example, *fcft_set_scaling_filter*().

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
*fcft_init*() does not matter, as long as it is done before calling
any other fcft API functions.

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

# RETURN VALUE

True if initialization was successful, otherwise false.

# SEE ALSO

*fcft_fini*()
