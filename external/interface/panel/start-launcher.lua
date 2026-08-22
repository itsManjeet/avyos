local awful     = require("awful")
local beautiful = require("beautiful")
local wibox     = require("wibox")
local gears     = require("gears")


local _M = {}

function _M.get()
    local start_menu = awful.popup({
        widget = {
            {
                {
                    text   = "foobar",
                    widget = wibox.widget.textbox,
                },
                {
                    {
                        text   = "foobar",
                        widget = wibox.widget.textbox,
                    },
                    bg     = "#FF00FF",
                    clip   = true,
                    shape  = gears.shape.rounded_bar,
                    widget = wibox.widget.background,
                },
                {
                    value         = 0.5,
                    forced_width  = 100,
                    forced_height = 30,
                    widget        = wibox.widget.progressbar,
                },
                layout = wibox.layout.fixed.vertical,
            },
            margins   = 10,
            widget    = wibox.container.margin,
            visible   = false,
            ontop     = true,
            placement = awful.placement.centered,
        },
    })

    local start_launcher = awful.widget.launcher({
        image = beautiful.awesome_icon,
        command = function()
            start_menu.visible = true
        end
    })

    return start_launcher
end

return setmetatable({}, { __call = function(_, ...) return _M.get(...) end })
