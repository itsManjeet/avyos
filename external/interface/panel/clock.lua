local wibox = require("wibox")

local _M = {}

function _M.get(s)
    local clock = wibox.widget.textclock()
    return clock
end

return setmetatable({}, { __call = function(_, ...) return _M.get(...) end })
