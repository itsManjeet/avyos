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

#ifndef arraylist_h
#define arraylist_h 1

#include <stdlib.h>  /* defines size_t, malloc(), realloc(), free(), abort() */
#include <new>
#if __cplusplus >= 201103L
# include <type_traits>
#endif

/* ArrayList<T> is a list of elements of type T, stored contiguously in memory
   (in order to make good use of memory caches in a CPU).
   T must be a type whose contents may be freely moved in memory (i.e. without
   fields that contain backpointers to the element itself).
   It is like std::vector<T>, but we don't want the bloated C++ standard
   library.
   It is like gnulib's "gl_list.hh" with the GL_ARRAY_LIST implementation.
   But here we don't want polymorphic containers (that force function calls at
   runtime); instead we want optimal inlining.  */

/* In order to avoid the need for explicit instantiation for each possible
   template parameter, we make the class ArrayList<T> entirely inline.
   The class ArrayListRepresentation is used to attach methods which we want
   to be compiled only once, avoiding duplicated code for each possible
   template parameter.  */

class ArrayListRepresentation
{
  template <class T>
    friend class ArrayList;
public:
  // ------------------------------ Constructors ------------------------------

  ArrayListRepresentation ()
  {
    _item = NULL;
    _nitems = 0;
    _nitems_max = 0;
  }

  // ------------------------------ Private stuff ------------------------------

private:
  /* Vector of entries.  */
  void *                _item;
  /* Number of elements of the vector that are used.  */
  size_t                _nitems;
  /* Number of elements of the vector that can be used at most.  */
  size_t                _nitems_max;

  /* Ensures that _nitems_max >= n.
     To be called only when _nitems_max < n.  */
  void                  ensure_capacity (size_t n, size_t size_of_T);
};

template <class T>
  class ArrayList
  {
  public:
    // ----------------------------- Constructors -----------------------------

    /* Creates a new ArrayList<T> with 0 elements,
       with no storage allocated initially.  */
    ArrayList ()
      : _rep () {}

    // ------------------------------ Destructor ------------------------------

    ~ArrayList ()
    {
      if (_rep._item != NULL)
        {
          /* Destruct the elements (in the opposite order of their
             initialization).  */
          #if __cplusplus >= 201103L
          /* See <https://en.cppreference.com/w/cpp/types/is_destructible>.  */
          if (! std::is_trivially_destructible<T>::value)
          #endif
            {
              size_t index = _rep._nitems;
              while (index > 0)
                {
                  --index;
                  (static_cast<T *>(_rep._item))[index].~T ();
                }
            }
          /* Free the storage.  */
          free (_rep._item);
        }
    }

    // ---------------------- Read-only member functions ----------------------

    /* Returns the current number of elements in the list.  */
    size_t size () const
    {
      return _rep._nitems;
    }

    T& get_at (size_t index) const
    {
      #if !__OPTIMIZE__
      if (index >= _rep._nitems)
        /* index out of range.  */
        abort ();
      #endif
      return (static_cast<T *>(_rep._item))[index];
    }

    // ---------------------- Modifying member functions ----------------------

    void set_at (size_t index, const T& value)
    {
      #if !__OPTIMIZE__
      if (index >= _rep._nitems)
        /* index out of range.  */
        abort ();
      #endif
      (static_cast<T *>(_rep._item))[index] = value;
    }

    size_t add_last (const T& value)
    {
      if (_rep._nitems == _rep._nitems_max)
        ensure_capacity (_rep._nitems + 1);
      size_t index = _rep._nitems;
      new (&(static_cast<T *>(_rep._item))[index]) T (value);
      _rep._nitems++;
      return index;
    }

    // ----------------------------- Private stuff -----------------------------

  private:
    /* Here, the vector of entries is a 'T *', but only the first _nitems
       elements are initialized.  The remaining memory is uninitialized.  */
    ArrayListRepresentation _rep;

    /* Ensures that _nitems_max >= n.
       To be called only when _nitems_max < n.  */
    void                ensure_capacity (size_t n)
    {
      _rep.ensure_capacity (n, sizeof (T));
    }
  };

#endif
