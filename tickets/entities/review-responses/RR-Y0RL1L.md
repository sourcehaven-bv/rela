---
id: RR-Y0RL1L
type: review-response
title: appDescription fallback regressed SettingsView
finding: The appDescription fallback overloaded AppConfig.Description (config + sidebar) with the metamodel's top-level description when app.description was empty. But AppConfig.Description is ALSO rendered by SettingsView as a plain one-line value — so the metamodel's (possibly multi-paragraph markdown) description would dump raw there. Unintended UI regression in a view the ticket never mentions.
severity: significant
resolution: 'Added a SEPARATE wire field v1.Config.about_description (helper renamed aboutDescription); AppConfig.Description restored to s.Cfg.App.Description only, so SettingsView is unchanged. New store field schemaStore.aboutDescription (from configData.about_description); StatusBar''s About reads it. Verified: config shows about_description populated + app.description empty.'
status: addressed
---
