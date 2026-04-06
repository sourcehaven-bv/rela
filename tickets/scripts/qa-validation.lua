#!/usr/bin/env -S rela flow
-- QA Test: Validation
-- Tests form validation rules

local test = rela.args[1] or "required"

if test == "required" then
    -- Test required field validation
    local event = rela.flow.emit({
        type = "form",
        title = "Required Field Test",
        fields = {
            {type = "markdown", content = "Try submitting without filling the required field:"},
            {name = "required_field", type = "text", label = "Required Field", required = true},
            {name = "optional_field", type = "text", label = "Optional Field"},
        },
        actions = {{"submit", "Submit"}},
    })
    rela.output({test = "required", data = event.data})

elseif test == "number" then
    -- Test number constraints
    local event = rela.flow.emit({
        type = "form",
        title = "Number Validation Test",
        fields = {
            {type = "markdown", content = "Enter numbers to test validation:"},
            {name = "any_number", type = "number", label = "Any Number"},
            {name = "positive", type = "number", label = "Positive (min=0)", min = 0},
            {name = "range", type = "number", label = "Range (1-10)", min = 1, max = 10},
            {name = "stepped", type = "number", label = "Stepped (step=5)", step = 5},
        },
        actions = {{"submit", "Submit"}},
    })
    rela.output({test = "number", data = event.data})

elseif test == "date" then
    -- Test date constraints
    local event = rela.flow.emit({
        type = "form",
        title = "Date Validation Test",
        fields = {
            {type = "markdown", content = "Enter dates to test validation:"},
            {name = "any_date", type = "date", label = "Any Date"},
            {name = "future", type = "date", label = "Future Only", min = rela.today},
            {name = "q1_2024", type = "date", label = "Q1 2024", min = "2024-01-01", max = "2024-03-31"},
        },
        actions = {{"submit", "Submit"}},
    })
    rela.output({test = "date", data = event.data})

else
    rela.output({error = "Unknown test: " .. test, available = {"required", "number", "date"}})
end
