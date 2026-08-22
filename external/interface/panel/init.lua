local awful = require("awful")
local wibox = require("wibox")

local widgets = {
    start_launcher = require("panel.start-launcher"),
    taglist        = require("panel.taglist"),
    tasklist       = require("panel.tasklist"),
    prompt         = require("panel.prompt"),
    clock          = require("panel.clock"),
}

local function setup_panel(s)
    s.wibox = awful.wibar({
        position = "bottom",
        height   = 54,
        screen   = s,
    })

    s.wibox:setup {
        layout = wibox.layout.align.horizontal,
        {
            widgets.start_launcher(),
            widgets.taglist(s),
            widgets.prompt(s),
            layout = wibox.layout.fixed.horizontal,
        },
        widgets.tasklist(s),
        {
            widgets.clock(s),
            layout = wibox.layout.fixed.horizontal,
        }
    }
end

awful.screen.connect_for_each_screen(function(s)
    setup_panel(s)
end)
