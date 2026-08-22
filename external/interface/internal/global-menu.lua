local awful         = require("awful")
local hotkeys_popup = require("awful.hotkeys_popup")


local _M = {}

function _M.get()
    local awesome_menu = {
        { "hotkeys", function() hotkeys_popup.show_help(nil, awful.screen.focused()) end },
        { "restart", awesome.restart },
    }

    local global_menu = awful.menu({
        items = {
            { "awesome",       awesome_menu },
            { "Open Terminal", RC.config.Terminal },
        },
    })

    return global_menu
end

return setmetatable({}, { __call = function(_, ...) return _M.get(...) end })
