// Package icondefs holds the canonical definition of the data-entry icon set:
// the one place a config-facing icon name, its Lucide component, its category
// and its description are written down.
//
// Everything else is generated from All() by `just generate-icons`:
//
//	icondefs.All()
//	     ├──► internal/dataentryconfig/icons_gen.go        (ValidIconNames)
//	     ├──► frontend/src/utils/iconRegistry.generated.ts (imports + ICONS)
//	     └──► docs-project/entities/guides/GUIDE-data-entry.md
//	                └──► docs/data-entry.md  (via `just docs`)
//
// # Why this is its own leaf package
//
// The generator imports this table and WRITES INTO internal/dataentryconfig. If
// both lived in one package, a hand-edit that broke the generated file would
// make the generator itself unbuildable — a bootstrap deadlock escapable only by
// hand-reverting the very file the tool exists to produce. This package contains
// no generated code, so it always compiles, so the generator always runs.
//
// # Adding an icon
//
// Append an IconDef and run `just generate-icons`. Do not hand-edit any
// generated file; a drift check fails the build if you do.
//
// Two rules for a new entry, both learned the expensive way:
//
//   - Name the GLYPH, not the use site. `wrench`, not `settings-page`. A name is
//     a permanent contract with every project that authored it, and a name tied
//     to one use site is wrong the moment a second use site appears.
//   - Prefer Lucide's CANONICAL component name over a legacy alias. Lucide still
//     exports aliases like `Home` and `AlertTriangle`, but they are absent from
//     its type declarations and disappear at a major bump. `House` and
//     `TriangleAlert` are the names that survive.
package icondefs

import "slices"

// NoIcon is the reserved name an author writes to suppress an icon entirely.
//
// It is NOT a member of All() — it names no component, and must never resolve to
// one. It is carried end to end as this literal string rather than being mapped
// to "" at any layer: the wire field is `json:"icon,omitempty"`, so an empty
// string vanishes from the payload and becomes indistinguishable from a field
// that was never set. Empty already means "use the kind-derived icon"; reusing
// it for "render nothing" would give one token opposite meanings on the two
// sides of the wire.
const NoIcon = "none"

// IconDef is one entry in the icon set.
type IconDef struct {
	// Name is the config-facing identifier authors write in data-entry.yaml.
	// Lowercase kebab-case. This is a public contract — renaming one breaks
	// every project that authored it.
	Name string

	// Lucide is the exported component name in lucide-vue-next, used verbatim
	// as an import identifier in the generated TypeScript. A typo here is a
	// compile error in the generated file, which is the point.
	Lucide string

	// Category groups the entry in the generated documentation table. Entries
	// are emitted in slice order, so keep a category's entries contiguous.
	Category string

	// Desc is a short noun phrase describing what the glyph depicts and when to
	// reach for it. Rendered into the docs table; required.
	Desc string
}

// spaChromeNames are the entries the SPA renders itself, outside any
// author-supplied config — the search box, the validation-warning link, the
// Apps section, the mobile footer, the theme toggle.
//
// The generator emits a named export for each so a rename breaks the TypeScript
// build. Without that, `ICONS.apps` on a Record<string, Component> is typed
// non-optional, so a missing key raises no type error and Vue renders nothing:
// the glyph silently disappears from the sidebar with nothing failing anywhere.
// The value is the TypeScript export identifier, written out rather than
// derived from the name. Deriving it would mean capitalizing an identifier in
// the generator, and the export is a name a human reads in an import statement
// — cheaper to state than to compute, and it keeps the generator free of
// string-casing logic.
var spaChromeNames = map[string]string{
	"search":   "IconSearch",
	"warning":  "IconWarning",
	"apps":     "IconApps",
	"settings": "IconSettings",
	"sun":      "IconSun",
	"moon":     "IconMoon",
}

// DerivedNames are the glyphs the SERVER picks for a navigation entry from its
// kind — a `list:` entry gets the list glyph, a `kanban:` the board glyph.
//
// A separate set from spaChromeNames because the coupling runs the other way
// and breaks differently. These names are chosen in Go
// (dataentry.navEntryToSidebarItem) and only ever RESOLVED in the SPA, so a
// rename cannot be caught by a TypeScript import: the handler would keep
// emitting the old string, `resolveIcon` would miss, and every entry of that
// kind would quietly render the fallback glyph. Exported so the handler can
// name them instead of repeating string literals, which is what makes the
// rename a compile error on the Go side too.
var DerivedNames = struct {
	Dashboard, List, Kanban, Calendar, Gantt, Search, Settings, Document, Action string
}{
	Dashboard: "dashboard",
	List:      "list",
	Kanban:    "kanban",
	Calendar:  "calendar",
	Gantt:     "gantt",
	Search:    "search",
	Settings:  "settings",
	Document:  "document",
	// An action entry historically derived NO glyph, which left `icon: none`
	// on one with nothing to fall back to when the sidebar collapses and hides
	// labels — so it borrowed the generic document glyph and read as a link to
	// a document while actually firing a mutation. It derives its own now.
	Action: "zap",
}

// IsChrome reports whether the SPA or the server references this name outside
// author config, which makes it non-renameable for a reason distinct from
// config compatibility. The generator refuses a table missing any of them.
func IsChrome(name string) bool {
	if _, ok := spaChromeNames[name]; ok {
		return true
	}
	return slices.Contains(derivedList(), name)
}

// SPAExport returns the TypeScript export identifier for a name the SPA imports
// directly, and "" for anything else. The server-derived names return "": they
// arrive as strings on the wire and are never imported.
func SPAExport(name string) string { return spaChromeNames[name] }

// derivedList returns the server-derived glyph names.
func derivedList() []string {
	d := DerivedNames
	return []string{d.Dashboard, d.List, d.Kanban, d.Calendar, d.Search, d.Settings, d.Document, d.Action}
}

// All returns the icon set in documentation order.
//
// Order is significant: it is the order of the generated docs table, and
// categories are expected to be contiguous. The generated Go and TypeScript
// artifacts sort by name independently, so their output does not depend on it.
//
// satisfy a length limit would scatter one list over a dozen functions and
// make the category grouping — which IS the documentation order — harder to
// read and to reorder, for no reduction in complexity.
//
//nolint:funlen // A data table, not logic. Splitting it across helpers to
func All() []IconDef {
	return []IconDef{
		// ── Navigation ──────────────────────────────────────────────────────
		{"dashboard", "House", "Navigation", "Home or landing view"},
		{"list", "List", "Navigation", "List view; the default list glyph"},
		{"kanban", "Kanban", "Navigation", "Board view; the default kanban glyph"},
		{"calendar", "CalendarDays", "Navigation", "Calendar view"},
		{"gantt", "ChartGantt", "Navigation", "Gantt view"},
		{"search", "Search", "Navigation", "Search"},
		{"settings", "Settings", "Navigation", "Settings or configuration"},
		{"apps", "Blocks", "Navigation", "Custom apps section"},
		{"document", "FileText", "Navigation", "Document view; also the fallback glyph"},
		{"table", "Table2", "Navigation", "Tabular data"},
		{"grid", "LayoutGrid", "Navigation", "Grid or tile layout"},
		{"layout", "LayoutDashboard", "Navigation", "Dashboard or panel layout"},
		{"rows", "Rows3", "Navigation", "Row-oriented layout"},
		{"columns", "Columns3", "Navigation", "Column-oriented layout"},
		{"menu", "Menu", "Navigation", "Menu or hamburger"},
		{"panel", "PanelLeft", "Navigation", "Side panel"},
		{"filter", "Funnel", "Navigation", "Filter or narrow a result set"},
		{"sliders", "SlidersHorizontal", "Navigation", "Adjustable settings or controls"},
		{"expand", "Maximize2", "Navigation", "Expand or maximize"},
		{"collapse", "Minimize2", "Navigation", "Collapse or minimize"},

		// ── Status and workflow ─────────────────────────────────────────────
		{"inbox", "Inbox", "Status & workflow", "Incoming or unstarted work"},
		{"status", "CircleDot", "Status & workflow", "Generic status; a dot in a circle"},
		{"done", "CircleCheck", "Status & workflow", "Completed or resolved"},
		{"check", "Check", "Status & workflow", "A bare check mark"},
		{"cancelled", "CircleX", "Status & workflow", "Cancelled or rejected"},
		{"blocked", "Ban", "Status & workflow", "Blocked or forbidden"},
		{"paused", "CirclePause", "Status & workflow", "Paused or on hold"},
		{"active", "CirclePlay", "Status & workflow", "Started; a play triangle in a circle"},
		{"progress", "LoaderCircle", "Status & workflow", "Work underway; a partial ring"},

		{"skipped", "CircleSlash", "Status & workflow", "Skipped or not applicable"},
		{"pending", "CircleEllipsis", "Status & workflow", "Awaiting input; an ellipsis in a circle"},
		{"warning", "TriangleAlert", "Status & workflow", "Warning or caution; a TRIANGLE"},
		{"alert", "CircleAlert", "Status & workflow", "Alert or error; a CIRCLE"},
		{"info", "Info", "Status & workflow", "Informational note"},
		{"help", "CircleQuestionMark", "Status & workflow", "Help or unanswered question"},
		{"flag", "Flag", "Status & workflow", "Flagged for attention"},
		{"star", "Star", "Status & workflow", "Favorite or highlight"},
		{"bookmark", "Bookmark", "Status & workflow", "Saved for later"},
		{"pin", "Pin", "Status & workflow", "Pinned to the top"},
		{"flame", "Flame", "Status & workflow", "Hot or high priority"},
		{"loading", "Loader", "Status & workflow", "In-flight or loading"},
		{"workflow", "Workflow", "Status & workflow", "A process or pipeline"},
		{"milestone", "Milestone", "Status & workflow", "A milestone or checkpoint"},
		{"target", "Target", "Status & workflow", "A goal or target"},

		// ── Time ────────────────────────────────────────────────────────────
		{"clock", "Clock", "Time", "A point in time"},
		{"timer", "Timer", "Time", "Elapsed or remaining time"},
		{"hourglass", "Hourglass", "Time", "Waiting or long-running"},
		{"history", "History", "Time", "Past revisions or an audit trail"},
		{"schedule", "CalendarClock", "Time", "A scheduled or recurring event"},
		{"calendar-check", "CalendarCheck", "Time", "A confirmed or completed date"},
		{"alarm", "AlarmClock", "Time", "A reminder or alarm"},
		{"sunrise", "Sunrise", "Time", "Start of day"},
		{"sunset", "Sunset", "Time", "End of day"},

		// ── People and organizations ────────────────────────────────────────
		{"user", "User", "People & orgs", "A single person"},
		{"users", "Users", "People & orgs", "A team or group"},
		{"user-add", "UserPlus", "People & orgs", "Add or invite a person"},
		{"user-check", "UserCheck", "People & orgs", "An approved or verified person"},
		{"user-remove", "UserX", "People & orgs", "Remove or deactivate a person"},
		{"contact", "Contact", "People & orgs", "A contact record"},
		{"organization", "Building2", "People & orgs", "A company or organization"},
		{"briefcase", "Briefcase", "People & orgs", "Work or a business matter"},
		{"handshake", "Handshake", "People & orgs", "An agreement or partnership"},
		{"award", "Award", "People & orgs", "A recognition or certification"},
		{"trophy", "Trophy", "People & orgs", "An achievement"},
		{"graduation", "GraduationCap", "People & orgs", "Training or education"},

		// ── Communication ───────────────────────────────────────────────────
		{"mail", "Mail", "Communication", "Email"},
		{"mail-open", "MailOpen", "Communication", "Read email"},
		{"message", "MessageSquare", "Communication", "A comment or single message"},
		{"discussion", "MessagesSquare", "Communication", "A thread or discussion"},
		{"send", "Send", "Communication", "Send or submit"},
		{"phone", "Phone", "Communication", "A phone call"},
		{"video", "Video", "Communication", "A video call or recording"},
		{"bell", "Bell", "Communication", "Notifications"},
		{"bell-active", "BellRing", "Communication", "An active or unread notification"},
		{"announce", "Megaphone", "Communication", "An announcement or broadcast"},
		{"feed", "Rss", "Communication", "A feed or subscription"},

		// ── Files and storage ───────────────────────────────────────────────
		{"file", "File", "Files & storage", "A generic file"},
		{"files", "Files", "Files & storage", "Several files"},
		{"file-add", "FilePlus", "Files & storage", "Create a file"},
		{"file-check", "FileCheck", "Files & storage", "An approved or validated file"},
		{"file-code", "FileCode", "Files & storage", "A source or config file"},
		{"spreadsheet", "FileSpreadsheet", "Files & storage", "A spreadsheet or tabular export"},
		{"folder", "Folder", "Files & storage", "A folder"},
		{"folder-open", "FolderOpen", "Files & storage", "An open folder"},
		{"hierarchy", "FolderTree", "Files & storage", "A hierarchy or nested structure"},
		{"archive", "Archive", "Files & storage", "Archived or inactive items"},
		{"package", "Package", "Files & storage", "A package or release"},
		{"box", "Box", "Files & storage", "A container or unit"},
		{"boxes", "Boxes", "Files & storage", "Inventory or a collection"},
		{"database", "Database", "Files & storage", "A database"},
		{"server", "Server", "Files & storage", "A server or host"},
		{"disk", "HardDrive", "Files & storage", "Storage or a disk"},
		{"cloud", "Cloud", "Files & storage", "A cloud service"},
		{"upload", "Upload", "Files & storage", "Upload"},
		{"download", "Download", "Files & storage", "Download"},
		{"attachment", "Paperclip", "Files & storage", "An attachment"},
		{"printer", "Printer", "Files & storage", "Print"},

		// ── Actions ─────────────────────────────────────────────────────────
		{"add", "Plus", "Actions", "Create or add"},
		{"remove", "Minus", "Actions", "Remove or subtract"},
		{"close", "X", "Actions", "Close or dismiss"},
		{"edit", "Pencil", "Actions", "Edit"},
		{"write", "PenLine", "Actions", "Compose or annotate"},
		{"erase", "Eraser", "Actions", "Clear or erase"},
		{"delete", "Trash2", "Actions", "Delete"},
		{"copy", "Copy", "Actions", "Copy"},
		{"clipboard", "Clipboard", "Actions", "Paste or clipboard"},
		{"checklist", "ClipboardList", "Actions", "A checklist or task list"},
		{"approve", "ClipboardCheck", "Actions", "Approve or sign off"},
		{"save", "Save", "Actions", "Save"},
		{"refresh", "RefreshCw", "Actions", "Refresh or re-run"},
		{"undo", "RotateCcw", "Actions", "Undo or revert"},
		{"play", "Play", "Actions", "Run or start"},
		{"pause", "Pause", "Actions", "Pause"},
		{"stop", "Square", "Actions", "Stop"},
		{"skip", "SkipForward", "Actions", "Skip ahead"},
		{"share", "Share2", "Actions", "Share"},
		{"link", "Link", "Actions", "A link or relation"},
		{"external", "ExternalLink", "Actions", "An external link"},
		{"cut", "Scissors", "Actions", "Cut or split"},

		// ── Security ────────────────────────────────────────────────────────
		{"lock", "Lock", "Security", "Locked or private"},
		{"unlock", "LockOpen", "Security", "Unlocked or public"},
		{"key", "KeyRound", "Security", "A key or credential"},
		{"shield", "Shield", "Security", "Protection or a control"},
		{"shield-check", "ShieldCheck", "Security", "A verified or passing control"},
		{"shield-alert", "ShieldAlert", "Security", "A failing or at-risk control"},
		{"visible", "Eye", "Security", "Visible or watched"},
		{"hidden", "EyeOff", "Security", "Hidden or redacted"},
		{"fingerprint", "FingerprintPattern", "Security", "Identity or authentication"},

		// ── Analysis ────────────────────────────────────────────────────────
		{"chart-bar", "ChartColumn", "Analysis", "A bar chart"},
		{"chart-line", "ChartLine", "Analysis", "A line chart or trend"},
		{"chart-pie", "ChartPie", "Analysis", "A pie chart or breakdown"},
		{"trending-up", "TrendingUp", "Analysis", "An increasing metric"},
		{"trending-down", "TrendingDown", "Analysis", "A decreasing metric"},
		{"activity", "Activity", "Analysis", "Activity or a live signal"},
		{"gauge", "Gauge", "Analysis", "A measure against a threshold"},
		{"calculator", "Calculator", "Analysis", "A calculation"},
		{"percent", "Percent", "Analysis", "A proportion"},
		{"scale", "Scale", "Analysis", "A balance or trade-off"},
		{"layers", "Layers", "Analysis", "Layers or grouping"},

		// ── Development ─────────────────────────────────────────────────────
		{"code", "Code", "Development", "Source code"},
		{"terminal", "Terminal", "Development", "A command or shell"},
		{"bug", "Bug", "Development", "A defect"},
		{"branch", "GitBranch", "Development", "A branch"},
		{"commit", "GitCommitHorizontal", "Development", "A commit"},
		{"merge", "GitMerge", "Development", "A merge"},
		{"pull-request", "GitPullRequest", "Development", "A pull request"},
		{"component", "Component", "Development", "A component or module"},
		{"puzzle", "Puzzle", "Development", "A plugin or extension"},
		{"cpu", "Cpu", "Development", "Compute or hardware"},
		{"network", "Network", "Development", "A network or topology"},
		{"wrench", "Wrench", "Development", "A tool or maintenance task"},
		{"hammer", "Hammer", "Development", "Building or construction"},
		{"cog", "Cog", "Development", "A mechanism or job"},
		{"flask", "FlaskConical", "Development", "A conical flask; an experiment or trial"},
		{"beaker", "Beaker", "Development", "A wide-mouthed beaker; research or measurement"},
		{"microscope", "Microscope", "Development", "Close inspection"},

		// ── Knowledge ───────────────────────────────────────────────────────
		{"book", "Book", "Knowledge", "A book or manual"},
		{"book-open", "BookOpen", "Knowledge", "A guide or open reference"},
		{"library", "Library", "Knowledge", "A collection of documents"},
		{"news", "Newspaper", "Knowledge", "News or an article"},
		{"scroll", "Scroll", "Knowledge", "A policy or long document"},
		{"note", "StickyNote", "Knowledge", "A short note"},
		{"notebook", "NotebookPen", "Knowledge", "A notebook or journal"},
		{"idea", "Lightbulb", "Knowledge", "An idea or proposal"},
		{"tag", "Tag", "Knowledge", "A single tag or label"},
		{"tags", "Tags", "Knowledge", "Several tags"},
		{"hash", "Hash", "Knowledge", "An identifier or channel"},

		// ── Places and logistics ────────────────────────────────────────────
		{"location", "MapPin", "Places & logistics", "A place"},
		{"map", "Map", "Places & logistics", "A map or overview"},
		{"globe", "Globe", "Places & logistics", "Global or international"},
		{"compass", "Compass", "Places & logistics", "Orientation or discovery"},
		{"route", "Route", "Places & logistics", "A route or path"},
		{"truck", "Truck", "Places & logistics", "Delivery or shipping"},
		{"plane", "Plane", "Places & logistics", "Air travel"},
		{"car", "Car", "Places & logistics", "Road travel"},
		{"train", "TramFront", "Places & logistics", "Rail travel"},
		{"ship", "Ship", "Places & logistics", "Sea freight or travel"},
		{"bed", "Bed", "Places & logistics", "Accommodation"},

		// ── Commerce ────────────────────────────────────────────────────────
		{"cart", "ShoppingCart", "Commerce", "An order or basket"},
		{"card", "CreditCard", "Commerce", "A payment method"},
		{"money", "Banknote", "Commerce", "Cash or an amount"},
		{"invoice", "Receipt", "Commerce", "An invoice or receipt"},
		{"wallet", "Wallet", "Commerce", "A budget or account"},
		{"price", "CircleDollarSign", "Commerce", "A price or cost"},
		{"gavel", "Gavel", "Commerce", "A legal or judicial matter"},

		// ── Devices and media ───────────────────────────────────────────────
		{"monitor", "Monitor", "Devices & media", "A desktop or display"},
		{"laptop", "Laptop", "Devices & media", "A laptop"},
		{"phone-device", "Smartphone", "Devices & media", "A mobile device"},
		{"tablet", "Tablet", "Devices & media", "A tablet"},
		{"image", "Image", "Devices & media", "An image"},
		{"camera", "Camera", "Devices & media", "A photo or capture"},
		{"mic", "Mic", "Devices & media", "Audio input"},
		{"music", "Music", "Devices & media", "Audio or music"},
		{"headphones", "Headphones", "Devices & media", "Listening or playback"},
		{"wifi", "Wifi", "Devices & media", "Connectivity"},
		{"battery", "Battery", "Devices & media", "Power level"},
		{"power", "Power", "Devices & media", "On or off"},
		{"plug", "Plug", "Devices & media", "An integration or connection"},

		// ── Health and environment ──────────────────────────────────────────
		{"heart", "Heart", "Health & environment", "A favorite or wellbeing"},
		{"pulse", "HeartPulse", "Health & environment", "Health or a live check"},
		{"stethoscope", "Stethoscope", "Health & environment", "A medical or diagnostic matter"},
		{"leaf", "Leaf", "Health & environment", "Sustainability"},
		{"tree", "TreePine", "Health & environment", "Nature or long-term growth"},
		{"sprout", "Sprout", "Health & environment", "Something new or growing"},
		{"recycle", "Recycle", "Health & environment", "Reuse or a cycle"},
		{"droplet", "Droplet", "Health & environment", "Liquid or a measurement"},
		{"temperature", "Thermometer", "Health & environment", "Temperature or intensity"},
		{"rain", "CloudRain", "Health & environment", "Rain or poor conditions"},
		{"snow", "Snowflake", "Health & environment", "Cold or frozen"},
		{"wind", "Wind", "Health & environment", "Wind or airflow"},

		// ── Theme ───────────────────────────────────────────────────────────
		{"sun", "Sun", "Theme", "Light mode"},
		{"moon", "Moon", "Theme", "Dark mode"},
		{"palette", "Palette", "Theme", "Colors or theming"},
		{"brush", "Brush", "Theme", "Styling or appearance"},
		{"contrast", "Contrast", "Theme", "Contrast or a display setting"},
		{"sparkles", "Sparkles", "Theme", "Something new, generated, or featured"},
		{"zap", "Zap", "Theme", "Fast or automated"},
		{"rocket", "Rocket", "Theme", "A launch or release"},
	}
}
