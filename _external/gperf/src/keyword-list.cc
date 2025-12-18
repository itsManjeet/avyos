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

#include <config.h>

/* Specification. */
#include "keyword-list.h"

#include <stddef.h>

/* ------------------------ Keyword_List<KT> class ------------------------- */

/* Constructor.  */
template <class KT>
  Keyword_List<KT>::Keyword_List (KT *car)
    : _cdr (NULL), _car (car)
  {
  }

/* ---------------------- Keyword_List<KT> functions ----------------------- */

/* Copies a linear list, sharing the list elements.  */
template <class KT>
  Keyword_List<KT> *
  copy_list (Keyword_List<KT> *list)
  {
    Keyword_List<KT> *result;
    Keyword_List<KT> **lastp = &result;
    while (list != NULL)
      {
        Keyword_List<KT> *new_cons = new Keyword_List<KT> (list->first());
        *lastp = new_cons;
        lastp = &new_cons->rest();
        list = list->rest();
      }
    *lastp = NULL;
    return result;
  }

/* Deletes a linear list, keeping the list elements in memory.  */
template <class KT>
  void
  delete_list (Keyword_List<KT> *list)
  {
    while (list != NULL)
      {
        Keyword_List<KT> *rest = list->rest();
        delete list;
        list = rest;
      }
  }

/* Type of a comparison function.  */
#if 0 /* wishful thinking */
template <class KT>
  typedef bool (*Keyword_Comparison) (KT *keyword1, KT *keyword2);
#endif

/* Merges two sorted lists together to form one sorted list.  */
template <class KT>
  static Keyword_List<KT> *
  merge (Keyword_List<KT> *list1, Keyword_List<KT> *list2,
         bool (*less) (KT *keyword1, KT *keyword2))
  {
    Keyword_List<KT> *result;
    Keyword_List<KT> **resultp = &result;
    for (;;)
      {
        if (!list1)
          {
            *resultp = list2;
            break;
          }
        if (!list2)
          {
            *resultp = list1;
            break;
          }
        if (less (list2->first(), list1->first()))
          {
            *resultp = list2;
            resultp = &list2->rest();
            /* We would have a stable sorting if the next line would read:
               list2 = *resultp;  */
            list2 = list1; list1 = *resultp;
          }
        else
          {
            *resultp = list1;
            resultp = &list1->rest();
            list1 = *resultp;
          }
      }
    return result;
  }

/* Sorts a linear list, given a comparison function.
   Note: This uses a variant of mergesort that is *not* a stable sorting
   algorithm.  */
template <class KT>
  Keyword_List<KT> *
  mergesort_list (Keyword_List<KT> *list,
                  bool (*less) (KT *keyword1, KT *keyword2))
  {
    if (list == NULL || list->rest() == NULL)
      /* List of length 0 or 1.  Nothing to do.  */
      return list;
    else
      {
        /* Determine a list node in the middle.  */
        Keyword_List<KT> *middle = list;
        for (Keyword_List<KT> *temp = list->rest();;)
          {
            temp = temp->rest();
            if (temp == NULL)
              break;
            temp = temp->rest();
            middle = middle->rest();
            if (temp == NULL)
              break;
          }

        /* Cut the list into two halves.
           If the list has n elements, the left half has ceiling(n/2) elements
           and the right half has floor(n/2) elements.  */
        Keyword_List<KT> *right_half = middle->rest();
        middle->rest() = NULL;

        /* Sort the two halves, then merge them.  */
        return merge (mergesort_list (list, less),
                      mergesort_list (right_half, less),
                      less);
      }
  }


#ifndef __OPTIMIZE__

#define INLINE /* not inline */
#include "keyword-list.icc"
#undef INLINE

#endif /* not defined __OPTIMIZE__ */

/* ------------------------------------------------------------------------- */

/* Explicit template instantiations.  Needed to avoid link-time errors.

   C++ is just misdesigned:  The most important aspect in building large
   software packages is information hiding.  (That's the point of having the
   implementation of a .h file in a .cc file, isn't it?  And of having
   classes with private fields and methods, isn't it?)  The fact that we
   need the instantiation of the Keyword_List<KT> class and associate functions
   only for KT = KeywordExt comes from the code in main.cc.  It is ugly that
   implementation details of main.cc have an influence into this file here.  */

template class Keyword_List<KeywordExt>;
template Keyword_List<KeywordExt> *
         copy_list<KeywordExt> (Keyword_List<KeywordExt> *list);
template void
         delete_list<KeywordExt> (Keyword_List<KeywordExt> *list);
template Keyword_List<KeywordExt> *
         mergesort_list<KeywordExt> (Keyword_List<KeywordExt> *list,
                                     bool (*less) (KeywordExt *keyword1,
                                                   KeywordExt *keyword2));

/* ------------------------------------------------------------------------- */
