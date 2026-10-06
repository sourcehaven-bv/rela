---
id: RR-4DMZOZ
type: review-response
title: editFormRoute builds a path instead of a named route
finding: 'A raw path is parsed, so an id with ? or # would split it. The previous EntityDetail code used the named form-edit route.'
severity: minor
resolution: 'editFormRoute returns {name: ''form-edit'', params, query}; tests updated.'
status: addressed
---
