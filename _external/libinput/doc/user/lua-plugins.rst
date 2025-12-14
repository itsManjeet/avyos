.. _lua_plugins:

==============================================================================
Lua Plugins
==============================================================================

libinput provides a plugin system that allows users to modify the behavior
of devices. For example, a plugin may add or remove axes and/or buttons on a
device and/or modify the event stream seen by this device before it is passed
to libinput.

Plugins are implemented in `Lua <https://www.lua.org/>`_ (version 5.4)
and are typically loaded from the following paths:

- ``/etc/libinput/plugins/*.lua``, and
- ``/usr/lib{64}/libinput/plugins/*.lua``

Plugins are loaded in alphabetical order and where
multiple plugins share the same file name, the one in the highest precedence
directory is used. Plugins in ``/etc`` take precedence over
plugins in ``/usr``.

.. note:: Plugins lookup paths and their order are decided by the compositor.
          Some compositors may support more/fewer/other lookup paths than the
          above defaults.

Plugins are run sequentially in ascending sort-order (i.e. ``00-foo.lua`` runs
before ``10-bar.lua``) and each plugin sees the state left by any previous
plugins. For example if ``00-foo.lua`` changes all left button events to right
button events, ``10-bar.lua`` only ever sees right button events.

See the `Lua Reference manual <https://www.lua.org/manual/5.4/manual.html>`_ for
details on the Lua language.

.. note:: Plugins are **not** loaded by default, it is up to the compositor
          whether to allow plugins. An explicit call to
          ``libinput_plugin_system_load_plugins()`` is required.

------------------------------------------------------------------------------
Limitations
------------------------------------------------------------------------------

Each script runs in its own sandbox and cannot communicate or share state with
other scripts.

Tables that hold API methods are not writable, i.e. it is not possible
to overwrite the default functionality of those APIs.

The Lua API available to plugins is limited to the following calls::

    assert  error   ipairs  next     pairs  tonumber
    pcall   select  print   tostring type   xpcall
    table   string  math    _VERSION

It is not possible to e.g. use the ``io`` module from a script.

To use methods on instantiated objects, the ``object:method`` method call
syntax must be used. For example:

.. code-block:: lua

    libinput:register()
    libinput.register() -- this will fail

------------------------------------------------------------------------------
When to use plugins
------------------------------------------------------------------------------

libinput plugins are a relatively niche use-case that typically need to
address either once-off issues (e.g. those caused by worn-out hardware) or
user preferences that libinput does not and will not cater for.

Plugins should not be used for issues that can be fixed generically, for
example via :ref:`device-quirks`.

As a rule of thumb: a plugin should be a once-off that only works for one
user's hardware. If a plugin can be shared with many users then the plugin
implements functionality that should be integrated into libinput proper.

------------------------------------------------------------------------------
Testing plugins
------------------------------------------------------------------------------

Our :ref:`tools` support plugins if passed the ``--enable-plugins`` commandline
option. For implementing and testing plugins the easiest commands to test are

- ``libinput debug-events --enable-plugins`` (see :ref:`libinput-debug-events` docs)
- ``libinput debug-gui --enable-plugins`` (see :ref:`libinput-debug-gui` docs)

Where libinput is built and run from git, the tools will also look for plugins
in the meson build directory. See the ``plugins/meson.build`` file for details.

.. _plugins_api_lua:

--------------------------------------------------------------------------------
Lua Plugin API
--------------------------------------------------------------------------------

Lua plugins sit effectively below libinput and the API is not a
representation of the libinput API. Plugins modify the evdev event stream
received from the kernel.

.. graphviz:: plugin-stack.gv

The API revolves around two types: ``libinput`` and ``EvdevDevice``. The
``libinput`` type is used to register a plugin from a script, the
``EvdevDevice`` represents one device that is present in the system (but may
not have yet been added by libinput).

Typically a script does the following steps:

- register with libinput via ``libinput:register({versions})``
- connect to the ``"new-evdev-device"`` event
- receive an ``EvdevDevice`` object in the ``"new-evdev-device"`` callback

  - check and/or modify the evdev event codes on the device
  - connect to the device's ``"evdev-frame"`` event

- receive an :ref:`evdev frame <plugins_api_evdev_frame>` in the device's
  ``"evdev-frame"`` callback

  - check and/or modify the events in that frame

Where multiple plugins are active, the evdev frame passed to the callback is
the combined frame as processed by all previous plugins in ascending sort order.
For example, if one plugin discards all button events subsequent plugins will
never see those button events in the frame.

.. _plugins_api_version_stability:

..............................................................................
Plugin version stability
..............................................................................

Plugin API version stability is provided on a best effort basis. We aim to provide
stable plugin versions for as long as feasible but may need to retire some older
versions over time. For this reason a plugin can select multiple versions it
implements, libinput will pick one supported version and adjust the plugin
behavior to match that version. See the ``libinput:register()`` call for details.

--------------------------------------------------------------------------------
Lua Plugin API Reference
--------------------------------------------------------------------------------


libinput provides the following globals and types:

.. _plugins_api_evdev_usage:

................................................................................
Evdev Usages
................................................................................

Evdev usages are a libinput-specific wrapper around the ``linux/input-event-codes.h``
evdev types and codes. They are used by libinput internally and are a 32-bit
combination of ``type << 16 | code``. Each usage carries the type and code and
is thus simpler to pass around and less prone to type confusion.

The :ref:`evdev global <plugins_api_evdev_global>` attempts to provide all
available usages but for the niche cases where it does not provide a named constant
the value can be crafted manually:

.. code-block:: lua

   evdev_type = 0x3  -- EV_REL
   evdev_code = 0x1  -- REL_Y
   evdev_usage = (evdev_type << 16) | evdev_code

   assert(usage == evdev.REL_Y)

.. _plugins_api_evdev_global:

................................................................................
The ``evdev`` global
................................................................................

The ``evdev`` global represents all known :ref:`plugins_api_evdev_usage`,
effectively in the form:

.. code-block:: lua

   evdev = {
      ABS_X = (3 << 16) | 0,
      ABS_Y = (3 << 16) | 1,
      ...
      REL_X = (2 << 16) | 0,
      REL_Y = (2 << 16) | 1,
      ...
   }


This global is provided for convenience to improve readability in the code.
Note that the name uses the event code name only (e.g. ``evdev.ABS_Y``) but the
value is an :ref:`Evdev Usage <plugins_api_evdev_usage>` (type and code).

See the ``linux/input-event-codes.h`` header file provided by your kernel
for a list of all evdev types and codes.

The evdev global also provides the bus type constants, e.g. ``evdev.BUS_USB``.
See the ``linux/input.h`` header file provided by your kernel
for a list of bus types.


.. _plugins_api_evdev_frame:

................................................................................
Evdev frames
................................................................................

Evdev frames represent a single frame of evdev events for a device. A frame
is a group of events that occurred at the same time. The frame usually only
contains state that has changed compared to the previous frame.

In our API a frame is exposed as a nested table with the following structure:

.. code-block:: lua

    frame1 = {
         { usage = evdev.ABS_X, value = 123 },
         { usage = evdev.ABS_Y, value = 456 },
         { usage = evdev.BTN_LEFT, value = 1 },
    }
    frame2 = {
         { usage = evdev.ABS_Y, value = 457 },
    }
    frame3 = {
         { usage = evdev.ABS_X, value = 124 },
         { usage = evdev.BTN_LEFT, value = 0 },
    }

.. note:: This API does not use ``SYN_REPORT`` events, it is implied at the
          end of the table. Where a plugin writes a ``SYN_REPORT`` into the
          list of events, that ``SYN_REPORT`` terminates the event frame
          (similar to writing a ``\0`` into the middle of a C string).
          A frame containing only a ``SYN_REPORT`` is functionally equivalent
          to an empty frame.

Events or frames do not have a timestamp. Where a timestamp is required, that
timestamp is passed as additional argument to the function or return value.

See :ref:`plugins_api_evdev_global` for a list of known usages.

.. warning:: Evdev frames have an implementation-defined size limit of how many
             events can be added to a single frame. This limit should never be
             hit by valid plugins.

.. _plugins_api_libinputglobal:

................................................................................
The ``libinput`` global object
................................................................................

The core of our plugin API is the ``libinput`` global object. A script must
immediately ``register()`` to be active, otherwise it is unloaded immediately.

All libinput-specific APIs can be accessed through the ``libinput`` object.

.. function:: libinput:register({1, 2, ...})

   Register this plugin with the given table of supported version numbers and
   returns the version number selected by libinput for this plugin. See
   :ref:`plugins_api_version_stability` for details.

   .. code-block:: lua

       -- this plugin can support versions 1, 4 and 5
       version = libinput:register({1, 4, 5})
       if version == 1 then
           ...

   This function must be the first function called.
   If the plugin calls any other functions before ``register()``, those functions
   return the default zero value for the return type (``nil``, ``0``, an empty
   table, etc.).

   If the plugin does not call ``register()`` it will be removed immediately.
   Once registered, any connected callbacks will be invoked whenever libinput
   detects new devices, removes devices, etc.

   This function must only be called once.

.. function:: libinput:unregister()

   Unregister this plugin. This removes the plugin from libinput and releases
   any resources associated with this plugin. This call must be the last call
   in your plugin, it is effectively equivalent to Lua's
   `os.exit() <https://www.lua.org/manual/5.4/manual.html#pdf-os.exit>`_.

.. function:: libinput:log_debug(message)

   Log a message at the libinput debug log priority. See
   ``libinput:log_error()`` for details.

.. function:: libinput:log_info(message)

   Log a message at the libinput info log priority. See
   ``libinput:log_error()`` for details.

.. function:: libinput:log_error(message)

   Log a message at the libinput error log priority. Whether a message is
   displayed in the log depends on libinput's log priority, set by the caller.

   A compositor may disable stdout and stderr. Log messages should be preferred
   over Lua's ``print()`` function to ensure the messages end up in the same
   location as other libinput log messages and are not discarded.

.. function:: libinput:now()

   Returns the current time in microseconds in ``CLOCK_MONOTONIC``. This is
   the timestamp libinput uses internally. This timestamp cannot be mapped
   to any particular time of day, see the
   `clock_gettime() man page <https://man7.org/linux/man-pages/man3/clock_gettime.3.html>`_
   for details.

.. function:: libinput:version()

   Returns the agreed-on version of the plugin, see ``libinput:register()``.
   If called before ``libinput:register()`` this function returns ``0``.

.. function:: libinput:connect(name, function)

   Set the callback to the given event name. Only one callback
   may be set for an event name at any time, subsequent callbacks
   will replace any earlier callbacks for the same name.

   Version 1 of the plugin API supports the following events and callback arguments:

   - ``"new-evdev-device"``: A new :ref:`EvdevDevice <plugins_api_evdevdevice>`
     has been seen by libinput but not yet added.

     .. code-block:: lua

      libinput:connect("new-evdev-device", function (device) ... end)

   - ``"timer-expired"``: The timer for this plugin has expired. This event is
     only sent if the plugin has set a timer with ``timer_set()``.

     .. code-block:: lua

      libinput:connect("timer-expired", function (now) ... end)

     The ``now`` argument is the current time in microseconds in
     ``CLOCK_MONOTONIC`` (see ``libinput:now()``).

.. function:: libinput:timer_cancel()

   Cancel the timer for this plugin. This is a no-op if the timer
   has not been set or has already expired.

.. function:: libinput:timer_set_absolute(time)

   Set a timer for this plugin, with the given time in microseconds.
   The timeout specifies an absolute time in microseconds (see
   ``libinput:now()``) The timer will expire once and then call the
   ``"timer-expired"`` event handler (if any).

   See ``libinput:timer_set_relative()`` for a relative timer.

   The following two lines of code are equivalent:

   .. code-block:: lua

      libinput:timer_set_relative(1000000) -- 1 second from now
      libinput:timer_set_absolute(libinput:now() + 1000000) -- 1 second from now

   Calling this function will cancel any existing (relative or absolute) timer.

.. function:: libinput:timer_set_relative(timeout)

   Set a timer for this plugin, with the given timeout in microseconds from
   the current time. The timer will expire once and then call the
   ``"timer-expired"`` event handler (if any).

   See ``libinput:timer_set_absolute()`` for an absolute timer.

   The following two lines of code are equivalent:

   .. code-block:: lua

      libinput:timer_set_relative(1000000) -- 1 second from now
      libinput:timer_set_absolute(libinput:now() + 1000000) -- 1 second from now

   Calling this function will cancel any existing (relative or absolute) timer.

.. _plugins_api_evdevdevice:

................................................................................
The ``EvdevDevice`` type
................................................................................

The ``EvdevDevice`` type represents a device available in the system
but not (yet) added by libinput. This device may be used to modify
a device's capabilities before the device is processed by libinput.

A plugin should always ``connect()`` to the ``"device-removed"`` callback
to be notified when a device is removed. If the plugin keeps a reference
to this device but the device is discarded by libinput, the device's query
methods will return zero values (e.g. ``nil``, ``0``, an empty table) and
methods will be noops.

.. function:: EvdevDevice:info()

   A table containing static information about the device, e.g.

   .. code-block:: lua

      {
         bustype = evdev.BUS_USB,
         vid = 0x1234,
         pid = 0x5678,
      }

   A plugin must ignore keys it does not know about.

   Version 1 of the plugin API supports the following keys and values:

   - ``bustype``: The numeric bustype of the device. See the
     ``BUS_*`` defines in ``linux/input.h`` for the list of possible values.
   - ``vid``: The 16-bit vendor ID of the device
   - ``pid``: The 16-bit product ID of the device

   If the device has since been discarded by libinput, this function returns an
   empty table.

.. function:: EvdevDevice:name()

   The device name as set by the kernel

.. function:: EvdevDevice:usages()

   Returns a table of all usages that are currently enabled for this
   device. Any type that exists on the device has a table assigned and in this
   table any code that exists on the device is a boolean true.
   For example:

   .. code-block:: lua

      {
         evdev.REL_X = true,
         evdev.REL_Y = true,
         evdev.BTN_LEFT = true,
      }

   All other usages are ``nil``, so that the following code is possible:

   .. code-block:: lua

      local usages = device:usages()
      if usages[evdev.REL_X] then
         -- do something
      end


   If the device has since been discarded by libinput, this function returns an
   empty table.

.. function:: EvdevDevice:absinfos()

   Returns a table of all ``EV_ABS`` codes that are currently enabled for this device.
   The event code is the key, each value is a table containing the following keys:
   ``minimum``, ``maximum``, ``fuzz``, ``flat``, ``resolution``.

   .. code-block:: lua

      {
         evdev.ABS_X = {
            minimum = 0,
            maximum = 1234,
            fuzz = 0,
            flat = 0,
            resolution = 45,
         },
      }

   If the device has since been discarded by libinput, this function returns an
   empty table.

.. function:: EvdevDevice:udev_properties()

   Returns a table containing a filtered list of udev properties available on this device
   in the form ``{ property_name = property_value, ... }``.
   udev properties used as a boolean (e.g. ``ID_INPUT``) are only present if their
   value is a logical true.

   Version 1 of the plugin API supports the following udev properties:

   - ``ID_INPUT`` and all of ``ID_INPUT_*`` that denote the device type as assigned
     by udev. This information is usually used by libinput to determine a
     device type. Note that for historical reasons these properties have
     varying rules - some properties may be mutually exclusive, others are
     independent, others may only be set if another property is set. Refer to
     the udev documentation (if any) for details. ``ID_INPUT_WIDTH_MM`` and
     ``ID_INPUT_HEIGHT_MM`` are excluded from this set.

   If the device has since been discarded by libinput, this function returns an
   empty table.

.. function:: EvdevDevice:enable_evdev_usage(usage)

   Enable the given :ref:`evdev usage <plugins_api_evdev_usage>` for this device.
   Use :ref:`plugins_api_evdev_global` for better readability,
   e.g. ``device:enable_evdev_usage(evdev.REL_X)``.
   This function must not be used for ``ABS_*`` events, use ``set_absinfo()``
   instead.

   Once a usage is enabled, events for that usage may be added to a device's
   frame.

   If the device has since been discarded by libinput, this function does nothing.

.. function:: EvdevDevice:disable_evdev_usage(usage)

   Disable the given :ref:`evdev usage <plugins_api_evdev_usage>` for this device.
   Use :ref:`plugins_api_evdev_global` for better readability,
   e.g. ``device:disable_evdev_usage(evdev.REL_X)``.

   Once a usage is disabled, events for that usage are discarded from any
   device frame.

   If the device has since been discarded by libinput, this function does nothing.

.. function:: EvdevDevice:set_absinfo(usage, absinfo)

   Set the absolute axis information for the given :ref:`evdev usage <plugins_api_evdev_usage>`
   and enable it if it does not yet exist on the device. The ``absinfo`` argument is a table
   containing zero or more of the following keys: ``minimum``, ``maximum``, ``fuzz``,
   ``flat``, ``resolution``. Any missing key defaults the corresponding
   value from the device if the device already has this event usage or zero otherwise.
   For example, the following code changes the resolution but leaves everything
   else as-is:

   .. code-block:: lua

      local absinfo = {
         resolution = 40,
      }
      device:set_absinfo(evdev.ABS_X, absinfo)
      device:set_absinfo(evdev.ABS_Y, absinfo)

   Use :ref:`plugins_api_evdev_global` for better readability as shown in the
   example above.

   If the device has since been discarded by libinput, this function does nothing.

   .. note:: Overriding the absinfo values often indicates buggy firmware. This should
             typically be fixed with an entry in the
             `60-evdev.hwdb <https://github.com/systemd/systemd/blob/main/hwdb.d/60-evdev.hwdb>`_
             or :ref:`device-quirks` instead of a plugin so all users of that
             device can benefit from the fix.

.. function:: EvdevDevice:connect(name, function)

   Set the callback to the given event name. Only one callback
   may be set for an event name at any time, subsequent callbacks
   will overwrite any earlier callbacks for the same name.

   If the device has since been discarded by libinput, this function does nothing.

   Version 1 of the plugin API supports the following events and callback arguments:

   - ``"evdev-frame"``: A new :ref:`evdev frame <plugins_api_evdev_frame>` has
     started for this device. If the callback returns a value other than
     ``nil``, that value is the frame with any modified events.
     An empty frame (``{}``) causes libinput to drop the current event frame.

     .. code-block:: lua

        device:connect("evdev-frame", function (device, frame, timestamp)
            -- change any event into a movement left by 1 pixel
            move_left = {
                  { usage = evdev.REL_X, value = -1, },
            }
            return move_left
        end

     The timestamp of an event frame is in microseconds in ``CLOCK_MONOTONIC``, see
     ``libinput:now()`` for details.

     For performance reasons plugins that do not modify the event frame should
     return ``nil`` (or nothing) instead of the event frame that was passed
     as argument.

   - ``"device-removed"``: This device was removed by libinput. This may happen
     without the device ever becoming a libinput device as seen by libinput's
     public API (e.g. if the device does not meet the requirements to be
     added). Once this callback is invoked, the plugin should remove any
     references to this device and stop using it.

     .. code-block:: lua

      device:connect("device-removed", function (device) ... end)

     Functions to query the device's capabilities (e.g. ``usages()``) will
     return an empty table.

.. function:: EvdevDevice:disconnect(name)

   Disconnect the existing callback (if any) for the given event name. See
   ``EvdevDevice:connect()`` for a list of supported names.

.. function:: EvdevDevice:prepend_frame(frame)

   Prepend an :ref:`evdev frame <plugins_api_evdev_frame>` for this device
   **before** the current frame (if any). The **next** plugin will see the
   prepended frame first followed by the current frame.

   This function can only be called from within a device's ``"evdev-frame"``
   handler or from within the plugin's timer callback function.

   For example, to change a single event into a drag, prepend a button
   down and append a button up before each event:

   .. code:: lua

      function frame_handler(device, frame, timestamp)
          device:prepend_frame({
              { usage = evdev.BTN_LEFT, value = 1}
          })
          device:append_frame({
              { usage = evdev.BTN_LEFT, value = 0}
          })
          return nil  -- return the current frame unmodified

          -- The next plugin sees the event sequence:
          --    button down, frame, button up
      end

   If called from within the plugin's timer there is no current frame and this
   function is identical to ``append_frame()``.

.. function:: EvdevDevice:append_frame(frame)

   Appends an :ref:`evdev frame <plugins_api_evdev_frame>` for this device
   **after** the current frame (if any). This function can only be called from
   within a device's ``"evdev-frame"`` handler or from within the plugin's timer
   callback function.

   If called from within the plugin's timer there is no current frame and this
   function is identical to ``prepend_frame()``.

   See ``prepend_frame()`` for more details.

.. function:: EvdevDevice:disable_feature(feature_name)

   Disable the given libinput-internal feature for this device. This should be used
   by plugins that replace that feature with a custom implementation for this device.

   libinput may have multiple internal implementations for any given feature, disabling
   it via this API disables any and all of those implementations, causing the feature to
   no longer work at all. It is up to the plugin implementation to re-implement that
   feature to match the user's expectation.

   Version 1 of the plugin API supports the following features:

   - ``"button-debouncing"``: see :ref:`button_debouncing`
   - ``"touchpad-hysteresis"``: see :ref:`touchpad_jitter`
   - ``"touchpad-jump-detection"``: see :ref:`touchpad_jumping_cursor`
   - ``"touchpad-palm-detection"``: see :ref:`palm_detection`
   - ``"wheel-debouncing"``: some high-resolution mouse wheel movements inside libinput
     are delayed and/or modified
