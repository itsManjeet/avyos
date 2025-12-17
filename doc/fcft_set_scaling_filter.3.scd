fcft_set_scaling_filter(3) "3.3.2" "fcft"

# NAME

fcft_set_scaling_filter - configures the filter to use when downscaling bitmap fonts

# SYNOPSIS

*\#include <fcft/fcft.h>*

*bool fcft_set_scaling_filter(enum fcft_scaling_filter *_filter_*);*

# DESCRIPTION

*fcft_set_scaling_filter*() configures the filter the use when
downscaling bitmap fonts (typically emoji fonts). The setting affects
*all* font instances.

This function does *not* clear the glyph caches and should therefore be
called before any calls to *fcft_codepoint_rasterize*().

Possible values for _filter_ are:

- *FCFT\_SCALING\_FILTER\_NONE*
- *FCFT\_SCALING\_FILTER\_NEAREST*
- *FCFT\_SCALING\_FILTER\_BILINEAR*
- *FCFT\_SCALING\_FILTER\_IMPULSE*
- *FCFT\_SCALING\_FILTER\_BOX*
- *FCFT\_SCALING\_FILTER\_LINEAR*
- *FCFT\_SCALING\_FILTER\_CUBIC*
- *FCFT\_SCALING\_FILTER\_GAUSSIAN*
- *FCFT\_SCALING\_FILTER\_LANCZOS2*
- *FCFT\_SCALING\_FILTER\_LANCZOS3*
- *FCFT\_SCALING\_FILTER\_LANCZOS3\_STRETCHED*

*FCFT\_SCALING\_FILTER\_NONE* disables filtering.

*FCFT\_SCALING\_FILTER\_NEAREST* and
*FCFT\_SCALING\_FILTER\_BILINEAR* are traditional filters, with
_nearest_ being the fastest.

The rest are separable convolution filters.

If this function is not called, fcft defaults to
*FCFT\_SCALING\_FILTER\_CUBIC*.

# RETURN VALUE

On success, *fcft_set_scaling_filter*() returns true. On error, false
is returned.

# EXAMPLE

See *fcft_from_name*()
