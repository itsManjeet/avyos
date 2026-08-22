local awful = require("awful")

local _M = {}

function _M.get(startups)
    for count = 1, #startups do
        awful.spawn.once(startups[count])
    end
end

return setmetatable({}, { __call = function(_, ...) return _M.get(...) end })
