/* This may look like C code, but it is really -*- C++ -*- */

/* Keyword list.

   Copyright (C) 2002-2003, 2025 Free Software Foundation, Inc.
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

#ifndef keyword_list_h
#define keyword_list_h 1

#include "keyword.h"

/* List node of a linear list of KT, where KT is a subclass of Keyword.  */
template <class KT>
  class Keyword_List
  {
  public:
    /* Constructor.  */
                          Keyword_List (KT *car);

    /* Access to first element of list.  */
    KT *                  first () const;
    /* Access to next element of list.  */
    Keyword_List<KT> *&   rest ();

  protected:
    Keyword_List<KT> *    _cdr;
    KT * const            _car;
  };

/* List node of a linear list of KeywordExt.  */
typedef Keyword_List<KeywordExt> KeywordExt_List;

/* Copies a linear list, sharing the list elements.  */
template <class KT>
  extern Keyword_List<KT> * copy_list (Keyword_List<KT> *list);

/* Deletes a linear list, keeping the list elements in memory.  */
template <class KT>
  extern void delete_list (Keyword_List<KT> *list);

/* Sorts a linear list, given a comparison function.
   Note: This uses a variant of mergesort that is *not* a stable sorting
   algorithm.  */
template <class KT>
  extern Keyword_List<KT> * mergesort_list (Keyword_List<KT> *list,
                                            bool (*less) (KT *keyword1,
                                                          KT *keyword2));

#ifdef __OPTIMIZE__

#define INLINE inline
#include "keyword-list.icc"
#undef INLINE

#endif

#endif
