---
id: RR-HY0G2Z
type: review-response
title: Fold key changes on every live append, so focus is lost
finding: 'Plan step 4 keys a fold by its last seq. A live progress frame that joins the tail run changes that key, so Vue destroys and recreates the fold li, its button and any expanded message rows inside it. A keyboard or screen-reader user on the fold button, or on a control in an expanded message, loses focus to body each time the runner posts. Progress posts arrive often while working, so the tail fold is the one most used. Change the plan: give the fold a Vue key that appends do not change (its first seq; only loadOlder and trimming at the top change that), and keep the scroll-anchor attribute as a separate value (see the data-seq finding). Add a channelFold or story case: focus on the tail fold button survives an append.'
severity: significant
resolution: A fold's Vue key is now fold-<first seq>, which a live append does not change, so focus on the fold or inside its run survives (step 4, AC 5). Added the ChannelsPanel case "focus on the tail fold survives an append" and a channelFold case for a stable key.
status: addressed
---
