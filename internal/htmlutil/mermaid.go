// Package htmlutil provides HTML post-processing utilities.
package htmlutil

import "regexp"

// MermaidBlockRe matches goldmark's output for mermaid fenced code blocks.
// Goldmark outputs: <pre><code class="language-mermaid">...</code></pre>
// Mermaid.js expects: <pre class="mermaid">...</pre>
var MermaidBlockRe = regexp.MustCompile(`<pre><code class="language-mermaid">([\s\S]*?)</code></pre>`)

// PlantUMLBlockRe matches goldmark's output for plantuml fenced code blocks.
// Goldmark outputs: <pre><code class="language-plantuml">...</code></pre>
// The SPA's renderPlantUMLDiagrams pass expects: <pre class="plantuml">...</pre>
// (the same shape as the mermaid rewrite — the frontend then turns it into an
// <img> pointed at the configured PlantUML server).
var PlantUMLBlockRe = regexp.MustCompile(`<pre><code class="language-plantuml">([\s\S]*?)</code></pre>`)

// ConvertMermaidBlocks transforms goldmark mermaid code blocks to mermaid.js format.
func ConvertMermaidBlocks(html string) string {
	return MermaidBlockRe.ReplaceAllString(html, `<pre class="mermaid">$1</pre>`)
}

// ConvertPlantUMLBlocks transforms goldmark plantuml code blocks to the
// <pre class="plantuml"> shape the SPA upgrades into a server-rendered diagram.
func ConvertPlantUMLBlocks(html string) string {
	return PlantUMLBlockRe.ReplaceAllString(html, `<pre class="plantuml">$1</pre>`)
}

// ConvertDiagramBlocks runs every diagram fence rewrite (mermaid, plantuml) in
// one call. Server-side markdown pipelines should call this rather than the
// individual converters so a newly added diagram type is picked up everywhere
// at once — the two-call-site drift this previously invited is the reason it
// exists.
func ConvertDiagramBlocks(html string) string {
	html = ConvertMermaidBlocks(html)
	html = ConvertPlantUMLBlocks(html)
	return html
}
