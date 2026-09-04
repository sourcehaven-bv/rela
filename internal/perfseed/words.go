package perfseed

// Word banks. Domain-flavored rather than lorem ipsum so that free-text
// search over the seeded corpus hits something recognizable, and so a
// title reads as a title in a screenshot.

var nouns = []string{
	"access", "audit", "backup", "baseline", "budget", "certificate", "change", "cluster",
	"compliance", "contract", "control", "dashboard", "deployment", "encryption", "endpoint",
	"evidence", "exception", "firewall", "gateway", "handover", "incident", "inventory",
	"key", "ledger", "logging", "migration", "monitoring", "network", "onboarding", "outage",
	"patch", "pipeline", "policy", "procedure", "recovery", "release", "retention", "review",
	"rollout", "runbook", "schema", "segment", "server", "session", "storage", "supplier",
	"telemetry", "tenant", "token", "training", "upgrade", "vault", "vendor", "workflow",
}

var adjectives = []string{
	"annual", "automated", "critical", "customer", "deferred", "external", "internal",
	"legacy", "manual", "mobile", "nightly", "primary", "quarterly", "regional", "remote",
	"secondary", "shared", "staging", "temporary", "weekly",
}

var verbs = []string{
	"align", "approve", "archive", "assess", "consolidate", "decommission", "document",
	"harden", "migrate", "monitor", "provision", "reconcile", "renew", "replace", "restore",
	"review", "rotate", "test", "validate", "verify",
}

var firstNames = []string{
	"Alma", "Bram", "Cato", "Daan", "Eline", "Femke", "Gijs", "Hanna", "Ilse", "Joost",
	"Kars", "Lotte", "Milan", "Noor", "Otto", "Pien", "Quirijn", "Roos", "Sem", "Tessa",
	"Ubbo", "Vera", "Wout", "Xavi", "Yara", "Zoë", "Anouk", "Bas", "Carlijn", "Dirk",
}

var lastNames = []string{
	"Bakker", "de Boer", "de Groot", "de Jong", "de Vries", "Dijkstra", "Hendriks",
	"Jansen", "Janssen", "Kok", "Meijer", "Mulder", "Peters", "Smit", "van Dijk",
	"van den Berg", "van der Meer", "Visser", "Vos", "Willems",
}

var roleTitles = []string{
	"Engineer", "Senior engineer", "Team lead", "Analyst", "Auditor", "Product owner",
	"Architect", "Operations specialist", "Security officer", "Controller",
}

var documentCategories = []string{"guide", "runbook", "reference", "decision"}
var controlFamilies = []string{"organizational", "people", "physical", "technological"}
var controlStatuses = []string{"missing", "partial", "implemented"}
var riskLevels = []string{"low", "medium", "high"}
var riskStatuses = []string{"open", "mitigated", "accepted"}
var taskStatuses = []string{"todo", "doing", "review", "done"}
var priorities = []string{"low", "medium", "high", "critical"}
var projectStatuses = []string{"proposed", "active", "on-hold", "done"}
var policyStatuses = []string{"to-do", "doing", "done"}

// Sentence fragments for body prose. Bodies are assembled from these so the
// markdown has real structure (headings, lists, a table) rather than one
// long paragraph — the render and search paths see something shaped like
// what people actually write.
var sentenceStarts = []string{
	"This item exists because", "The owning team agreed that", "During the last review it became clear that",
	"For the coming period", "As a precondition", "The audit found that", "Operationally,",
	"To keep the risk register current,", "Where the supplier is involved,", "Before sign-off,",
}

var sentenceMiddles = []string{
	"the %s %s must be %sd", "every %s %s is %sd", "no %s %s may be %sd", "the %s %s was %sd",
	"a %s %s should be %sd", "the team will %s the %s %s", "we %s each %s %s", "someone has to %s the %s %s",
}

var sentenceEnds = []string{
	"before the next release.", "within the agreed window.", "and the outcome is recorded here.",
	"unless an exception is approved.", "so the evidence stays current.", "with the change logged.",
	"and reported to the owner.", "at the end of the quarter.", "without manual steps.", "as documented.",
}
