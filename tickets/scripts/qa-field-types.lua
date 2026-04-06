#!/usr/bin/env -S rela flow
-- QA Test: All Field Types
-- Tests each field type with various options

local event = rela.flow.emit({
    type = "form",
    title = "Field Types QA Test",
    description = "Testing all supported field types",
    fields = {
        {type = "markdown", content = "## Text Fields"},
        {name = "text_simple", type = "text", label = "Simple Text"},
        {name = "text_required", type = "text", label = "Required Text", required = true},
        {name = "text_default", type = "text", label = "With Default", default = "default value"},
        {name = "text_placeholder", type = "text", label = "With Placeholder", placeholder = "Enter something..."},
        {name = "text_multiline", type = "text", label = "Multiline (3 lines)", lines = 3},

        {type = "markdown", content = "## Select Fields"},
        {name = "select_simple", type = "select", label = "Simple Select",
         options = {{"a", "Option A"}, {"b", "Option B"}, {"c", "Option C"}}},
        {name = "select_default", type = "select", label = "With Default",
         options = {{"low", "Low"}, {"medium", "Medium"}, {"high", "High"}}, default = "medium"},

        {type = "markdown", content = "## Multi-Select Fields"},
        {name = "multi_simple", type = "multi-select", label = "Multi Select",
         options = {{"tag1", "Tag 1"}, {"tag2", "Tag 2"}, {"tag3", "Tag 3"}}},

        {type = "markdown", content = "## Other Fields"},
        {name = "bool_field", type = "boolean", label = "Boolean Toggle"},
        {name = "bool_default", type = "boolean", label = "Boolean (default true)", default = true},
        {name = "number_simple", type = "number", label = "Number"},
        {name = "number_constrained", type = "number", label = "Number (1-100)", min = 1, max = 100},
        {name = "date_simple", type = "date", label = "Date"},
        {name = "date_constrained", type = "date", label = "Date (2024)", min = "2024-01-01", max = "2024-12-31"},
    },
    actions = {
        {"submit", "Submit", "primary"},
        {"cancel", "Cancel", "warning"},
    },
})

if event.action == "cancel" then
    rela.output({cancelled = true})
    return
end

rela.output({
    action = event.action,
    data = event.data,
})
