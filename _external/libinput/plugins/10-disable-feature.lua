-- SPDX-License-Identifier: MIT
--
-- An example plugin to show how to disable an internal feature.
--
-- Typically one would expect the plugin to re-implement the feature
-- in a more device-specific manner but that's not done here.

-- UNCOMMENT THIS LINE TO ACTIVATE THE PLUGIN
-- libinput:register({1})
libinput:connect("new-evdev-device", function(device)
    local udev_info = device:udev_properties()
    if udev_info["ID_INPUT_TOUCHPAD"] then
        libinput:log_info("Disabling palm detection on " .. device:name())
        device:disable_feature("touchpad-palm-detection")
    end
end)
