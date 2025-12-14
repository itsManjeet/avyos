-- SPDX-License-Identifier: MIT
--
-- An example plugin to make the pointer go three times as fast
--
-- Install this file in /etc/libinput/plugins and
--
-- UNCOMMENT THIS LINE TO ACTIVATE THE PLUGIN
-- libinput:register({1})
libinput:connect("new-evdev-device", function(device)
    local usages = device:usages()
    if usages[evdev.REL_X] then
        device:connect("evdev-frame", function(device, frame, timestamp)
            for _, v in ipairs(frame) do
                if v.usage == evdev.REL_X or v.usage == evdev.REL_Y then
                    -- Multiply the relative motion by 3
                    v.value = v.value * 3
                end
            end
            return frame
        end)
    end
end)
