-- SPDX-License-Identifier: MIT
--
-- This is an example libinput plugin
--
-- This plugin maps a downwards mouse wheel to a button down event and
-- an upwards wheel movement to a button up event.

-- UNCOMMENT THIS LINE TO ACTIVATE THE PLUGIN
-- libinput:register({1})

-- The button we want to press on wheel events
local wheel_button = evdev.BTN_EXTRA
local button_states = {}

local function evdev_frame(device, frame, timestamp)
    local events = {}
    local modified = false

    for _, v in ipairs(frame) do
        if v.usage == evdev.REL_WHEEL then
            -- REL_WHEEL is inverted, neg value -> down, pos value -> up
            if v.value < 0 then
                if not button_states[device] then
                    table.insert(events, { usage = wheel_button, value = 1 })
                    button_states[device] = true
                end
            else
                if button_states[device] then
                    table.insert(events, { usage = wheel_button, value = 0 })
                    button_states[device] = false
                end
            end
            modified = true
        -- Because REL_WHEEL is no longer a wheel, the high-res
        -- events are dropped
        elseif v.usage == evdev.REL_WHEEL_HI_RES then
            modified = true
        else
            table.insert(events, v)
        end
    end

    if modified then
        return events
    else
        return nil
    end
end

local function device_new(device)
    local usages = device:usages()
    if usages[evdev.REL_WHEEL] then
        button_states[device] = false
        if not usages[wheel_button] then
            device:enable_evdev_usage(wheel_button)
        end
        device:connect("evdev-frame", evdev_frame)
        device:connect("device-removed", function(dev)
            button_states[dev] = nil
        end)
    end
end

libinput:connect("new-evdev-device", device_new)
