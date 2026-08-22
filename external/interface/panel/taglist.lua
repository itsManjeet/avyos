local awful = require("awful")
local gears = require("gears")

local _M = {}

function _M.get(s)
    s.taglist = awful.widget.taglist {
        screen = s,
        filter = awful.widget.taglist.filter.all,
        buttons = gears.table.join(
            awful.button({}, 1, function(t) t:view_only() end),
            awful.button({ RC.config.ModKey }, 1, function(t)
                if client.focus then
                    client.focus:move_to_tag(t)
                end
            end)
        )
    }
    return s.taglist
end

return setmetatable({}, { __call = function(_, ...) return _M.get(...) end })
