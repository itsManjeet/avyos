-- SPDX-License-Identifier: MIT
--
-- An example plugin to make the pointer go three times as slow
--
-- Install this file in /etc/libinput/plugins and
--
-- UNCOMMENT THIS LINE TO ACTIVATE THE PLUGIN
-- libinput:register({1})
remainders = {}

function split(v)
    if math.abs(v) >= 1.0 then
        local i = math.floor(math.abs(v))
        local r = math.abs(v) % 1.0
        if v < 0.0 then
            i = -i
            r = -r
        end
        return i, r
    else
        return 0, v
    end
end

function decelerate(device, x, y)
    local remainder = remainders[device]
    local rx, ry = 0, 0
    if x ~= 0.0 then
        rx, remainder.x = split(remainder.x + x/3.0)
    end
    if y ~= 0.0 then
        ry, remainder.y = split(remainder.y + y/3.0)
    end

    return rx, ry
end

libinput:connect("new-evdev-device", function(device)
    local usages = device:usages()
    if usages[evdev.REL_X] then
        remainders[device] = { x = 0.0, y = 0.0 }
        device:connect("evdev-frame", function(device, frame, timestamp)
            for _, v in ipairs(frame) do
                if v.usage == evdev.REL_X then
                    v.value, _ = decelerate(device, v.value, 0.0)
                elseif v.usage == evdev.REL_Y then
                    _, v.value = decelerate(device, 0.0, v.value)
                end
            end
            return frame
        end)
        device:connect("device-removed", function(dev)
            remainders[dev] = nil
        end)
    end
end)
