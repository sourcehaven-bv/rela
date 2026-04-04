-- Validate that wont-fix/deferred review responses have adequate justification.
-- Called from metamodel validation rules with entity pre-set.
--
-- Arguments (via rela.args):
--   [1] = minimum character count required
--
-- Returns: nil (pass) or {message=...} (violation)

local min_chars = tonumber(rela.args[1]) or 100

local status = entity.properties.status or ""
if status ~= "wont-fix" and status ~= "deferred" then
    return nil  -- Rule only applies to wont-fix/deferred
end

local reason = entity.properties.reason or ""
if #reason < min_chars then
    return {
        message = "Justification too short (got " .. #reason .. " chars, need " .. min_chars .. "+)"
    }
end

return nil
