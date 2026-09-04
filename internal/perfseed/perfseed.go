package perfseed

import (
	"fmt"
	"hash/fnv"
	"iter"
	"math/rand/v2"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store/storeutil"
)

// Profile sizes one generated graph: how many entities of each type, and
// how often the faced types carry a second face.
type Profile struct {
	Teams, People, Projects, Tasks, Controls, Risks, Policies, Documents int

	// PublishedShare is the fraction of policies that also have a
	// `published` face; DutchShare the fraction of documents with an `nl`
	// face. Both in [0, 1].
	PublishedShare, DutchShare float64
}

// Perf is the profile behind `rela dev seed --profile perf`: at scale 1
// roughly 20k entity rows (faces included) and 45k relations, in the
// proportions of a mid-sized organisation's portfolio plus ISMS. Scale
// multiplies every count; it is clamped so a typo cannot ask for a
// million rows.
func Perf(scale float64) Profile {
	scale = min(max(scale, 0.001), 10)
	n := func(base int) int { return max(1, int(float64(base)*scale+0.5)) }
	return Profile{
		// Never fewer than the named teams: acl.yaml and the demo people
		// refer to them by id, so a tiny scale must still produce them.
		Teams:          max(len(teamNames), n(8)),
		People:         n(800),
		Projects:       n(300),
		Tasks:          n(11000),
		Controls:       n(400),
		Risks:          n(1200),
		Policies:       n(1500),
		Documents:      n(2000),
		PublishedShare: 0.6,
		DutchShare:     0.5,
	}
}

// Generator produces the entities and relations of one profile under one
// seed. Both streams are independent pure functions of the seed (see the
// package doc), so they may be consumed in either order or twice.
type Generator struct {
	p    Profile
	seed uint64
}

// New returns a generator for p under seed.
func New(p Profile, seed uint64) *Generator {
	return &Generator{p: p, seed: seed}
}

// Profile returns the sizing the generator was built with.
func (g *Generator) Profile() Profile { return g.p }

// The graph is dated relative to a fixed origin so two runs on different
// days still produce identical rows.
var origin = time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)

// Team names are fixed rather than generated: acl.yaml assigns roles to
// team ids, so the ids must be knowable before the data exists. Beyond the
// eight named teams, extra teams (at scale > 1) get numbered engineering ids
// and no role.
var teamNames = []struct{ id, title, department string }{
	{"TEAM-leadership", "Leadership", "leadership"},
	{"TEAM-security", "Security office", "security"},
	{"TEAM-engineering-1", "Platform engineering", "engineering"},
	{"TEAM-engineering-2", "Product engineering", "engineering"},
	{"TEAM-engineering-3", "Data engineering", "engineering"},
	{"TEAM-operations-1", "Site operations", "operations"},
	{"TEAM-operations-2", "Service desk", "operations"},
	{"TEAM-finance", "Finance & control", "finance"},
}

// Demo principals, one per role in acl.yaml. They are the first three
// people so a tiny scale still produces them.
var demoPeople = []struct{ name, email, team string }{
	{"Alice Manager", "alice@perf.example", "TEAM-leadership"},
	{"Bob Editor", "bob@perf.example", "TEAM-engineering-1"},
	{"Carol Reader", "carol@perf.example", "TEAM-finance"},
}

// PRNG stream families. Each (kind, index) pair owns one stream; the
// relation streams use kindRel with an index offset per relation family so
// a project's edges and a task's edges never share a sequence.
const (
	kindTeam = iota + 1
	kindPerson
	kindProject
	kindTask
	kindControl
	kindRisk
	kindPolicy
	kindDocument
	kindBody // a separate stream per entity so body length never shifts property draws
	kindRel
)

const (
	relPeople    = 1_000_000
	relProjects  = 2_000_000
	relTasks     = 3_000_000
	relControls  = 4_000_000
	relPolicies  = 5_000_000
	relDocuments = 6_000_000
)

// rng returns the PRNG for one (kind, index) pair: the same pair always
// yields the same sequence, and no two pairs share one.
func (g *Generator) rng(kind, i int) *rand.Rand {
	h := fnv.New64a()
	fmt.Fprintf(h, "%d:%d:%d", g.seed, kind, i)
	return rand.New(rand.NewPCG(g.seed, h.Sum64()))
}

// IDs are prefix plus zero-padded index: unique by construction, sortable,
// and valid under entity.ValidateID (letters, digits, one dash).
func teamID(i int) string {
	if i < len(teamNames) {
		return teamNames[i].id
	}
	return fmt.Sprintf("TEAM-engineering-%d", i-len(teamNames)+4)
}
func personID(i int) string   { return fmt.Sprintf("PERS-%05d", i+1) }
func projectID(i int) string  { return fmt.Sprintf("PRJ-%04d", i+1) }
func taskID(i int) string     { return fmt.Sprintf("TSK-%05d", i+1) }
func controlID(i int) string  { return fmt.Sprintf("CTL-%04d", i+1) }
func riskID(i int) string     { return fmt.Sprintf("RSK-%04d", i+1) }
func policyID(i int) string   { return fmt.Sprintf("POL-%04d", i+1) }
func documentID(i int) string { return fmt.Sprintf("DOC-%04d", i+1) }

// Face decisions are the one thing both streams must agree on, so they are
// derived from the (kind, index) PRNG's first draw and nothing else.
func (g *Generator) policyPublished(i int) bool {
	return g.rng(kindPolicy, i).Float64() < g.p.PublishedShare
}
func (g *Generator) documentDutch(i int) bool {
	return g.rng(kindDocument, i).Float64() < g.p.DutchShare
}

// projectWindow is the project's planned span; tasks are dated inside it,
// so both the project and its tasks derive it from the project's PRNG.
func (g *Generator) projectWindow(i int) (start, end, target time.Time) {
	r := g.rng(kindProject, i)
	start = origin.AddDate(0, 0, r.IntN(300))
	end = start.AddDate(0, 0, 30+r.IntN(210))
	target = end.AddDate(0, 0, r.IntN(41)-20)
	return start, end, target
}

// projectParent returns the containing project for a subproject, or -1.
// Only earlier projects qualify, so containment is acyclic.
func (g *Generator) projectParent(i int) int {
	r := g.rng(kindRel, relProjects+i)
	if i == 0 || r.Float64() >= 0.2 {
		return -1
	}
	return r.IntN(i)
}

// taskParent returns (projectIndex, taskIndex): a task belongs to one
// project directly (taskIndex == -1) or as a subtask of an earlier task in
// the same project.
func (g *Generator) taskParent(i int) (project, task int) {
	r := g.rng(kindRel, relTasks+i)
	project = r.IntN(g.p.Projects)
	if i > 0 && r.Float64() < 0.15 {
		// Pick an earlier task and use its project so the tree stays inside
		// one project. Recomputing the earlier task's parent is a draw on
		// that task's own stream, which is deterministic.
		t := r.IntN(i)
		p, _ := g.taskParent(t)
		return p, t
	}
	return project, -1
}

func dateStr(t time.Time) string { return t.Format("2006-01-02") }

func pick[T any](r *rand.Rand, xs []T) T { return xs[r.IntN(len(xs))] }

func titleCase(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func (g *Generator) title(r *rand.Rand) string {
	return titleCase(pick(r, verbs)) + " " + pick(r, adjectives) + " " + pick(r, nouns) + " " + pick(r, nouns)
}

// body writes a markdown body of roughly the requested size (500 B – 8 KB)
// from the sentence banks: an intro paragraph, then sections with prose, a
// bullet list, and occasionally a table.
func (g *Generator) body(kind, i int, title string) string {
	r := g.rng(kindBody, kind*1_000_000+i)
	target := 500 + r.IntN(7500)
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", title)
	writeParagraph(&b, r)
	for b.Len() < target {
		fmt.Fprintf(&b, "## %s %s\n\n", titleCase(pick(r, adjectives)), pick(r, nouns))
		writeParagraph(&b, r)
		switch r.IntN(4) {
		case 0:
			for range 2 + r.IntN(5) {
				fmt.Fprintf(&b, "- %s the %s %s\n", titleCase(pick(r, verbs)), pick(r, adjectives), pick(r, nouns))
			}
			b.WriteString("\n")
		case 1:
			b.WriteString("| Item | Owner | Due |\n| --- | --- | --- |\n")
			for range 2 + r.IntN(4) {
				fmt.Fprintf(&b, "| %s %s | %s | %s |\n", pick(r, adjectives), pick(r, nouns),
					pick(r, firstNames), dateStr(origin.AddDate(0, 0, r.IntN(365))))
			}
			b.WriteString("\n")
		}
	}
	return b.String()
}

func writeParagraph(b *strings.Builder, r *rand.Rand) {
	for range 2 + r.IntN(5) {
		b.WriteString(pick(r, sentenceStarts))
		b.WriteByte(' ')
		mid := pick(r, sentenceMiddles)
		if strings.HasPrefix(mid, "the team") || strings.HasPrefix(mid, "we ") || strings.HasPrefix(mid, "someone") {
			fmt.Fprintf(b, mid, pick(r, verbs), pick(r, adjectives), pick(r, nouns))
		} else {
			fmt.Fprintf(b, mid, pick(r, adjectives), pick(r, nouns), pick(r, verbs))
		}
		b.WriteByte(' ')
		b.WriteString(pick(r, sentenceEnds))
		b.WriteByte(' ')
	}
	b.WriteString("\n\n")
}

func newEntity(id, typ string, face entity.Face) *entity.Entity {
	e := entity.New(id, typ)
	e.Face = face
	return e
}

// The two non-default faces this profile writes. Built through the codec
// once, at init, so the generator never string-converts a Face itself.
var (
	facePublished = mustFace("published")
	faceNL        = mustFace("nl")
)

func mustFace(s string) entity.Face {
	f, err := entity.ParseFace(s)
	if err != nil {
		panic(err)
	}
	return f
}

// Entities yields every entity row in creation order: teams, people,
// projects, tasks, controls, risks, policies (draft then, when present,
// published), documents (en then nl). Each row's id passes
// storeutil.ValidateID; the generator panics otherwise, since a bad id is a
// generator bug rather than a data condition.
func (g *Generator) Entities() iter.Seq[*entity.Entity] {
	return func(yield func(*entity.Entity) bool) {
		for _, seq := range []iter.Seq[*entity.Entity]{
			g.teams(), g.people(), g.projects(), g.tasks(),
			g.controls(), g.risks(), g.policies(), g.documents(),
		} {
			for e := range seq {
				if err := storeutil.ValidateID(e.ID); err != nil {
					panic("perfseed: generated invalid id: " + err.Error())
				}
				if !yield(e) {
					return
				}
			}
		}
	}
}

// each yields one entity per index in [0, n).
func each(n int, build func(i int) *entity.Entity) iter.Seq[*entity.Entity] {
	return func(yield func(*entity.Entity) bool) {
		for i := range n {
			if !yield(build(i)) {
				return
			}
		}
	}
}

func (g *Generator) teams() iter.Seq[*entity.Entity] {
	return each(g.p.Teams, func(i int) *entity.Entity {
		r := g.rng(kindTeam, i)
		e := newEntity(teamID(i), "team", "")
		if i < len(teamNames) {
			e.SetString("title", teamNames[i].title)
			e.SetString("department", teamNames[i].department)
		} else {
			e.SetString("title", titleCase(pick(r, adjectives))+" engineering")
			e.SetString("department", "engineering")
		}
		e.Content = g.body(kindTeam, i, e.GetString("title"))
		return e
	})
}

func (g *Generator) people() iter.Seq[*entity.Entity] {
	return each(g.p.People, func(i int) *entity.Entity {
		r := g.rng(kindPerson, i)
		e := newEntity(personID(i), "person", "")
		if i < len(demoPeople) {
			e.SetString("title", demoPeople[i].name)
			e.SetString("email", demoPeople[i].email)
		} else {
			e.SetString("title", pick(r, firstNames)+" "+pick(r, lastNames))
			e.SetString("email", fmt.Sprintf("person%05d@perf.example", i+1))
		}
		e.SetString("role_title", pick(r, roleTitles))
		e.Properties["salary"] = 38000 + r.IntN(90)*1000
		e.SetString("started", dateStr(origin.AddDate(-r.IntN(10), -r.IntN(12), 0)))
		e.Content = g.body(kindPerson, i, e.GetString("title"))
		return e
	})
}

func (g *Generator) projects() iter.Seq[*entity.Entity] {
	return each(g.p.Projects, func(i int) *entity.Entity {
		r := g.rng(kindProject, i)
		start, end, target := g.projectWindow(i)
		e := newEntity(projectID(i), "project", "")
		e.SetString("title", g.title(r))
		e.SetString("status", pick(r, projectStatuses))
		e.SetString("planned_start", dateStr(start))
		e.SetString("planned_end", dateStr(end))
		e.SetString("target_date", dateStr(target))
		e.Properties["budget"] = (5 + r.IntN(400)) * 1000
		e.Content = g.body(kindProject, i, e.GetString("title"))
		return e
	})
}

func (g *Generator) tasks() iter.Seq[*entity.Entity] {
	return each(g.p.Tasks, func(i int) *entity.Entity {
		r := g.rng(kindTask, i)
		project, _ := g.taskParent(i)
		pStart, pEnd, _ := g.projectWindow(project)
		span := int(pEnd.Sub(pStart).Hours() / 24)
		start := pStart.AddDate(0, 0, r.IntN(max(span, 1)))
		due := start.AddDate(0, 0, 1+r.IntN(30))
		e := newEntity(taskID(i), "task", "")
		e.SetString("title", g.title(r))
		e.SetString("status", pick(r, taskStatuses))
		e.SetString("priority", pick(r, priorities))
		e.SetString("start", dateStr(start))
		e.SetString("due", dateStr(due))
		e.Properties["estimate"] = 1 + r.IntN(40)
		e.Content = g.body(kindTask, i, e.GetString("title"))
		return e
	})
}

func (g *Generator) controls() iter.Seq[*entity.Entity] {
	return each(g.p.Controls, func(i int) *entity.Entity {
		r := g.rng(kindControl, i)
		e := newEntity(controlID(i), "control", "")
		e.SetString("title", titleCase(pick(r, adjectives))+" "+pick(r, nouns)+" control")
		e.SetString("family", pick(r, controlFamilies))
		e.SetString("status", pick(r, controlStatuses))
		e.Content = g.body(kindControl, i, e.GetString("title"))
		return e
	})
}

func (g *Generator) risks() iter.Seq[*entity.Entity] {
	return each(g.p.Risks, func(i int) *entity.Entity {
		r := g.rng(kindRisk, i)
		e := newEntity(riskID(i), "risk", "")
		e.SetString("title", "Risk of "+pick(r, adjectives)+" "+pick(r, nouns)+" "+pick(r, nouns))
		e.SetString("level", pick(r, riskLevels))
		e.Properties["likelihood"] = 1 + r.IntN(5)
		e.Properties["impact"] = 1 + r.IntN(5)
		e.SetString("status", pick(r, riskStatuses))
		e.Content = g.body(kindRisk, i, e.GetString("title"))
		return e
	})
}

// policies yields each policy's draft row and, for the published share,
// its published face right after it.
func (g *Generator) policies() iter.Seq[*entity.Entity] {
	return func(yield func(*entity.Entity) bool) {
		for i := range g.p.Policies {
			r := g.rng(kindPolicy, i)
			_ = r.Float64() // the published decision (policyPublished) — keep draws aligned
			title := titleCase(pick(r, adjectives)) + " " + pick(r, nouns) + " policy"
			draft := newEntity(policyID(i), "policy", "")
			draft.SetString("title", title)
			draft.SetString("status", pick(r, policyStatuses))
			draft.SetString("owner", pick(r, firstNames)+" "+pick(r, lastNames))
			draft.SetString("version", fmt.Sprintf("%d.%d", 1+r.IntN(3), r.IntN(10)))
			draft.Content = g.body(kindPolicy, i, title+" (draft)")
			if !yield(draft) {
				return
			}
			if !g.policyPublished(i) {
				continue
			}
			pub := newEntity(policyID(i), "policy", facePublished)
			pub.SetString("title", title)
			pub.SetString("status", "done")
			pub.SetString("owner", draft.GetString("owner"))
			pub.SetString("version", fmt.Sprintf("%d.0", 1+r.IntN(3)))
			pub.Content = g.body(kindPolicy, g.p.Policies+i, title)
			if !yield(pub) {
				return
			}
		}
	}
}

// documents yields each document's English row and, for the Dutch share,
// its nl face right after it.
func (g *Generator) documents() iter.Seq[*entity.Entity] {
	return func(yield func(*entity.Entity) bool) {
		for i := range g.p.Documents {
			r := g.rng(kindDocument, i)
			_ = r.Float64() // the Dutch-face decision (documentDutch)
			title := titleCase(pick(r, verbs)) + " " + pick(r, adjectives) + " " + pick(r, nouns)
			en := newEntity(documentID(i), "document", "")
			en.SetString("title", title)
			en.SetString("category", pick(r, documentCategories))
			en.Content = g.body(kindDocument, i, title)
			if !yield(en) {
				return
			}
			if !g.documentDutch(i) {
				continue
			}
			nl := newEntity(documentID(i), "document", faceNL)
			nl.SetString("title", title+" (NL)")
			nl.SetString("category", en.GetString("category"))
			nl.Content = g.body(kindDocument, g.p.Documents+i, title+" (NL)")
			if !yield(nl) {
				return
			}
		}
	}
}

// Relation is one edge to create. FromFace is zero for identity-scoped
// edges and for content-scoped edges from the default face.
type Relation struct {
	From, Type, To string
	FromFace       entity.Face
}

// Relations yields every edge, after all entities it references exist in
// the Entities order. Endpoints are chosen from earlier indices where the
// relation is self-referential (contains, depends-on), so the graph is
// acyclic where the app assumes it (the gantt hierarchy).
func (g *Generator) Relations() iter.Seq[Relation] {
	return func(yield func(Relation) bool) {
		for _, seq := range []iter.Seq[Relation]{
			g.membership(), g.projectEdges(), g.taskEdges(),
			g.controlEdges(), g.policyEdges(), g.documentEdges(),
		} {
			for r := range seq {
				if !yield(r) {
					return
				}
			}
		}
	}
}

// edgesPerIndex yields the edges emit produces for each index in [0, n).
// emit appends to the slice it is handed and returns it.
func edgesPerIndex(n int, emit func(i int, out []Relation) []Relation) iter.Seq[Relation] {
	return func(yield func(Relation) bool) {
		var buf []Relation
		for i := range n {
			buf = emit(i, buf[:0])
			for _, r := range buf {
				if !yield(r) {
					return
				}
			}
		}
	}
}

// distinct draws up to n indices below limit with r, without repeats.
func distinct(r *rand.Rand, n, limit int) []int {
	seen := make(map[int]bool, n)
	var out []int
	for range n {
		v := r.IntN(limit)
		if seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}

func (g *Generator) membership() iter.Seq[Relation] {
	return edgesPerIndex(g.p.People, func(i int, out []Relation) []Relation {
		r := g.rng(kindRel, relPeople+i)
		team := teamID(r.IntN(g.p.Teams))
		if i < len(demoPeople) {
			team = demoPeople[i].team
		}
		return append(out, Relation{From: personID(i), Type: "member-of", To: team})
	})
}

func (g *Generator) projectEdges() iter.Seq[Relation] {
	return edgesPerIndex(g.p.Projects, func(i int, out []Relation) []Relation {
		r := g.rng(kindRel, relProjects+i)
		_ = r.Float64() // projectParent's draw, kept aligned
		out = append(out,
			Relation{From: teamID(r.IntN(g.p.Teams)), Type: "owns", To: projectID(i)},
			Relation{From: personID(r.IntN(g.p.People)), Type: "leads", To: projectID(i)},
		)
		if parent := g.projectParent(i); parent >= 0 {
			out = append(out, Relation{From: projectID(parent), Type: "contains", To: projectID(i)})
		}
		return out
	})
}

func (g *Generator) taskEdges() iter.Seq[Relation] {
	return edgesPerIndex(g.p.Tasks, func(i int, out []Relation) []Relation {
		project, parentTask := g.taskParent(i)
		parent := projectID(project)
		if parentTask >= 0 {
			parent = taskID(parentTask)
		}
		out = append(out, Relation{From: parent, Type: "contains", To: taskID(i)})
		r := g.rng(kindRel, relTasks+g.p.Tasks+i)
		if r.Float64() < 0.9 {
			out = append(out, Relation{From: taskID(i), Type: "assigned-to", To: personID(r.IntN(g.p.People))})
		}
		if i > 0 {
			for _, dep := range distinct(r, r.IntN(3), i) {
				out = append(out, Relation{From: taskID(i), Type: "depends-on", To: taskID(dep)})
			}
		}
		return out
	})
}

func (g *Generator) controlEdges() iter.Seq[Relation] {
	return edgesPerIndex(g.p.Controls, func(i int, out []Relation) []Relation {
		r := g.rng(kindRel, relControls+i)
		for _, risk := range distinct(r, 1+r.IntN(4), g.p.Risks) {
			out = append(out, Relation{From: controlID(i), Type: "mitigates", To: riskID(risk)})
		}
		return out
	})
}

func (g *Generator) policyEdges() iter.Seq[Relation] {
	return edgesPerIndex(g.p.Policies, func(i int, out []Relation) []Relation {
		r := g.rng(kindRel, relPolicies+i)
		out = append(out, Relation{From: policyID(i), Type: "owned-by", To: teamID(r.IntN(g.p.Teams))})
		for _, c := range distinct(r, 1+r.IntN(4), g.p.Controls) {
			out = append(out, Relation{From: policyID(i), Type: "implements", To: controlID(c)})
		}
		if g.policyPublished(i) {
			// The published face implements a partly different set — the
			// point of a content-scoped edge.
			for _, c := range distinct(r, 1+r.IntN(3), g.p.Controls) {
				out = append(out, Relation{From: policyID(i), Type: "implements", To: controlID(c), FromFace: facePublished})
			}
		}
		return out
	})
}

func (g *Generator) documentEdges() iter.Seq[Relation] {
	return edgesPerIndex(g.p.Documents, func(i int, out []Relation) []Relation {
		r := g.rng(kindRel, relDocuments+i)
		refs := func(face entity.Face) {
			seen := map[string]bool{}
			for range 1 + r.IntN(3) {
				to := controlID(r.IntN(g.p.Controls))
				if r.Float64() < 0.6 {
					to = policyID(r.IntN(g.p.Policies))
				}
				if seen[to] {
					continue
				}
				seen[to] = true
				out = append(out, Relation{From: documentID(i), Type: "references", To: to, FromFace: face})
			}
		}
		refs("")
		if g.documentDutch(i) {
			refs(faceNL)
		}
		return out
	})
}
