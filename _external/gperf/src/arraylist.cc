/* This may look like C code, but it is really -*- C++ -*- */

/* Array-list container.

   Copyright (C) 2025 Free Software Foundation, Inc.
   Written by Bruno Haible <bruno@clisp.org>.

   This file is part of GNU GPERF.

   This program is free software: you can redistribute it and/or modify
   it under the terms of the GNU General Public License as published by
   the Free Software Foundation; either version 3 of the License, or
   (at your option) any later version.

   This program is distributed in the hope that it will be useful,
   but WITHOUT ANY WARRANTY; without even the implied warranty of
   MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
   GNU General Public License for more details.

   You should have received a copy of the GNU General Public License
   along with this program.  If not, see <https://www.gnu.org/licenses/>.  */

#include <config.h>

/* Specification. */
#include "arraylist.h"

#include <new>

void ArrayListRepresentation::ensure_capacity (size_t n, size_t size_of_T)
{
  size_t new_max = 2 * _nitems_max + 1;
  if (new_max < n)
    new_max = n;
  void *new_item = realloc (_item, new_max * size_of_T);
  if (new_item == NULL)
    throw std::bad_alloc();
  /* The realloc() call has moved the elements from the old storage to the
     new storage. The old storage is thus now considered uninitialized.  */
  _item = new_item;
  _nitems_max = new_max;
}
