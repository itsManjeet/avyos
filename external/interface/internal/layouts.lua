local awful = require("awful")

local _M = {}

function _M.get()
    local layouts = {
        awful.layout.suit.floating,
        awful.layout.suit.tile,
    }

    awful.screen.connect_for_each_screen(function(s)
        awful.tag({ "1", "2" }, s, layouts[1])
    end)
    return layouts
end

return setmetatable({}, { __call = function(_, ...) return _M.get(...) end })
