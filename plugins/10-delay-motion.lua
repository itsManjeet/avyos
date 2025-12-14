-- SPDX-License-Identifier: MIT
--
-- This is an example libinput plugin
--
-- This plugin delays any event with relative motion by the given DELAY
-- by storing it in a table and replaying it via a timer callback later.

-- UNCOMMENT THIS LINE TO ACTIVATE THE PLUGIN
-- libinput:register({1})

DELAY = 1500 * 1000 -- 1.5s
next_timer_expiry = 0
devices = {}

function timer_expired(time_in_microseconds)
    next_timer_expiry = 0
    for device, frames in pairs(devices) do
        while #frames > 0 and frames[1].time <= time_in_microseconds do
            --- we don't have a current frame so it doesn't matter
            --- whether we prepend or append
            device:prepend_frame(frames[1].frame)
            table.remove(frames, 1)
        end
        local next_frame = frames[1]
        if next_frame and (next_timer_expiry == 0 or next_frame.time < next_timer_expiry) then
            next_timer_expiry = next_frame.time
        end
    end
    if next_timer_expiry ~= 0 then
        libinput:timer_set_absolute(next_timer_expiry)
    end
end

function frame(device, frame, timestamp)
    for _, v in ipairs(frame) do
        if v.usage == evdev.REL_X or v.usage == evdev.REL_Y then
            local next_time = timestamp + DELAY
            table.insert(devices[device], {
                time = next_time,
                frame = frame
            })
            if next_timer_expiry == 0 then
                next_timer_expiry = next_time
                libinput:timer_set_absolute(next_timer_expiry)
            end
            return {} -- discard frame
        end
    end
    return nil
end

function device_new(device)
    local usages = device:usages()
    if usages[evdev.REL_X] then
        devices[device] = {}
        device:connect("evdev-frame", frame)
        device:connect("device-removed", function(dev)
            devices[dev] = nil
        end)
    end
end

libinput:connect("new-evdev-device", device_new)
libinput:connect("timer-expired", timer_expired)
