local awful = require("awful")
local gears = require("gears")

local _M = {}

function _M.get(s)
    s.prompt = awful.widget.prompt()
    return s.prompt
end

return setmetatable({}, { __call = function(_, ...) return _M.get(...) end })
