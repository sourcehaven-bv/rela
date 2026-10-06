// Package desktopnotify decides which native desktop notifications the rela
// desktop app shows, and what number its Dock badge carries.
//
// The operator declares both in desktop.yaml at the project root. Each
// notification rule names an entity type and a predicate condition. An
// entity of that type that satisfies the condition is a match, and each match
// becomes one notification with a title and body rendered from the entity.
// The optional badge rule counts the entities that satisfy its condition.
//
// The package has three parts, used in this order:
//
//  1. [Load] reads desktop.yaml through the project's config loader and
//     compiles every condition and template. A bad file fails here, once,
//     rather than on every evaluation.
//  2. [Config.Evaluate] reads the entities from the store and returns every
//     current match plus the badge count.
//  3. [Tracker.Update] compares the matches with the ones it has seen before
//     and returns only the new ones. Those are the notifications to show.
//
// The tracker remembers the matched entities per rule in a state.KV. When an
// entity stops matching it is forgotten, so it notifies again if it matches
// again later.
//
// The first evaluation of a rule is silent. When the tracker has no record
// for a rule (the first launch, or a rule the operator just added), it
// records every current match as seen and returns none of them. Without this,
// opening a project with fifty overdue tasks would show fifty notifications at
// once. Notifications are meant to report changes, and the first evaluation
// has nothing to compare against. The badge is not affected: it always shows
// the current count.
//
// The package does not import the desktop shell's UI toolkit. The shell owns
// the timer, the store subscription and the native calls; this package only
// computes what to show.
package desktopnotify
