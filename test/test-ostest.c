/* Copyright (C) 2025 fontconfig Authors */
/* SPDX-License-Identifier: HPND */

#include <assert.h>
#include <fontconfig/fontconfig.h>

int
main (void)
{
    FcObjectSet *os = FcObjectSetCreate();

    FcObjectSetAdd (os, "test");
    FcObjectSetAdd (os, "test");
    FcObjectSetAdd (os, "test");
    FcObjectSetAdd (os, "test");
    FcObjectSetAdd (os, "test");

    assert (os->nobject == 0);
    assert (os->nobjIds == 1);

    FcObjectSetAdd (os, FC_FAMILY);

    assert (os->nobject == 0);
    assert (os->nobjIds == 2);

    FcObjectSetDestroy (os);

    return 0;
}
