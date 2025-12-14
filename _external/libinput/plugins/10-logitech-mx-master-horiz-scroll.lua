-- SPDX-License-Identifier: MIT
--
-- This plugin inverts the horizontal scroll direction of
-- the Logitech MX Master mouse. OOTB the mouse scrolls
-- in the opposite direction to all other mice out there.
--
-- This plugin is only needed when the mouse is connected
-- to the Logitech Bolt receiver - on that receiver we cannot
-- tell which device is connected.
--
-- For the Logitech Unifying receiver and Bluetooth please
-- add the ModelInvertHorizontalScrolling=1 quirk
-- in quirks/30-vendor-logitech.quirks.
--
--
-- Install this file in /etc/libinput/plugins and
--
-- UNCOMMENT THIS LINE TO ACTIVATE THE PLUGIN
-- libinput:register({1})
libinput:connect("new-evdev-device", function (device)
    local info = device:info()
    if info.vid == 0x046D and info.pid == 0xC548 then
        device:connect("evdev-frame", function (device, frame, timestamp)
            for _, event in ipairs(frame) do
                if event.usage == evdev.REL_HWHEEL or event.usage == evdev.REL_HWHEEL_HI_RES then
                    event.value = -event.value
                end
            end
            return frame
        end)
    end
end)
