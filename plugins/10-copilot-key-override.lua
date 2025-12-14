-- SPDX-License-Identifier: MIT
--
-- This is an example libinput plugin
--
-- This plugin detects the Copilot key on the keyboard with
-- the given VID/PID and replaces it with a different key (sequence).

-- UNCOMMENT THIS LINE TO ACTIVATE THE PLUGIN
-- libinput:register({1})

-- Replace this with your keyboard's VID/PID
KEYBOARD_VID = 0x046d
KEYBOARD_PID = 0x4088

meta_is_down = false
shift_is_down = false

-- shift-A, because you can never have enough screaming
replacement_sequence = { evdev.KEY_LEFTSHIFT, evdev.KEY_A }

function frame(device, frame, _)
    for _, v in ipairs(frame) do
        if v.value ~= 2 then -- ignore key repeats
            if v.usage == evdev.KEY_LEFTMETA then
                meta_is_down = v.value == 1
            elseif v.usage == evdev.KEY_LEFTSHIFT then
                shift_is_down = v.value == 1
            elseif v.usage == evdev.KEY_F23 and meta_is_down and shift_is_down then
                -- We know from the MS requirements that F23 for copilot is
                -- either last key (on press) or the first key (on release)
                -- of the three-key sequence, and no other keys are
                -- within this frame.
                if v.value == 1 then
                    -- Release our modifiers first
                    device:prepend_frame({
                        { usage = evdev.KEY_LEFTSHIFT, value = 0 },
                        { usage = evdev.KEY_LEFTMETA, value = 0 },
                    })
                    -- Insert our replacement press sequence
                    local replacement_frame = {}
                    for _, rv in ipairs(replacement_sequence) do
                        table.insert(replacement_frame, { usage = rv, value = 1 })
                    end
                    device:append_frame(replacement_frame)
                else
                    -- Insert our replacement release sequence
                    local replacement_frame = {}
                    for idx = #replacement_sequence, 1, -1 do
                        table.insert(replacement_frame, { usage = replacement_sequence[idx], value = 0 })
                    end
                    device:append_frame(replacement_frame)

                    -- we don't care about re-pressing shift/meta because the
                    -- rest of the stack will filter the release for an
                    -- unpressed key anyway.
                end

                return {} -- discard this frame
            end
        end
    end
end

function device_new(device)
    local info = device:info()
    if  info.vid == KEYBOARD_VID and info.pid == KEYBOARD_PID then
        device:connect("evdev-frame", frame)
    end
end

libinput:connect("new-evdev-device", device_new)
