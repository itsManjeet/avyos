-- SPDX-License-Identifier: MIT
--
-- This is an example libinput plugin
--
-- This plugin swaps left and right buttons on any device that has both buttons.

-- Let's create a plugin. A single Lua script may create more than one
-- plugin instance but that's a bit of a nice use-case. Most scripts
-- should be a single plugin.

-- A plugin needs to be registered to activate. If it isn't, it is
-- cleaned up immediately. In the register call we supply
-- the list of plugin versions we support. Currently we only
-- have version 1.
--
-- UNCOMMENT THIS LINE TO ACTIVATE THE PLUGIN
-- libinput:register({1})

-- Note to the reader: it will be easier to understand this example
-- if you read it bottom-up from here on.

-- The callback for our "evdev-frame" signal.
-- These frames are sent *before* libinput gets to handle them so
-- any modifications will affect libinput.
function frame(device, frame, time_in_microseconds)
    -- Frame is a table in the form
    -- { { usage: 123, value: 3 }, ... }
    -- Let's use the evdev module to make it more readable, evdev.KEY_A
    -- is simply the value (0x1 << 16) | 0x1 (see linux/input-event-codes.h)
    for _, v in ipairs(frame) do
        -- If we get a right button event, change it to left, and
        -- vice versa. Because this happens before libinput (or the next
        -- plugin in the precedence order) sees the
        -- frame it doesn't know that the swap happened.
        if v.usage == evdev.BTN_RIGHT then
            v.usage = evdev.BTN_LEFT
        elseif v.usage == evdev.BTN_LEFT then
            v.usage = evdev.BTN_RIGHT
        end
    end
    -- We changed the frame, let's return it. If we return nil
    -- the original frame is being used as-is.
    return frame
end

-- This is a timer callback. It is invoked after one second
-- (see below) but only once - re-set the timer so
-- it goes off every second.
function timer_expired(time_in_microseconds)
    libinput:timer_set_absolute(time_in_microseconds + 1000000)
end

-- Callback for the "new-evdev-device" signal, see below
-- The argument is the EvdevDevice object, see the documentation.
function device_new(device)
    -- A table of evdev usages available on our device.
    -- Using the evdev module makes it more readable but you can
    -- use numbers (which is what evdev.EV_KEY and
    -- friends resolve to anyway).
    local usages = device:usages()
    if usages[evdev.BTN_LEFT] and usages[evdev.BTN_RIGHT] then
        -- The "evdev-frame" callback is invoked whenever the device
        -- provided us with one evdev frame, i.e. a bunch of events up
        -- to excluding EV_SYN SYN_REPORT.
        device:connect("evdev-frame", frame)
    end

    -- The device has udev information, let's print it out. Right
    -- now all we get are the ID_INPUT_ bits.
    -- If this is empty we know libinput will ignore this device anyway
    local udev_info = device:udev_properties()
    for k, v in pairs(udev_info) do
        libinput:log_debug(k .. "=" .. v)
    end
end

-- Let's connect to the "new-evdev-device" signal. This function
-- is invoked when libinput detects a new evdev device (but before
-- that device is actually available to libinput as libinput device).
-- This allows us to e.g. change properties on the device.
libinput:connect("new-evdev-device", device_new)

-- Set our timer to expire 1s from now (in microseconds).
-- Timers are absolute, so they need to be added to the
-- current time
libinput:connect("timer-expired", timer_expired)
libinput:timer_set_relative(1000000)
