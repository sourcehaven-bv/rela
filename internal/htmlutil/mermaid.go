// Package htmlutil provides HTML post-processing utilities.
package htmlutil

import "regexp"

// MermaidBlockRe matches goldmark's output for mermaid fenced code blocks.
// Goldmark outputs: <pre><code class="language-mermaid">...</code></pre>
// Mermaid.js expects: <pre class="mermaid">...</pre>
var MermaidBlockRe = regexp.MustCompile(`<pre><code class="language-mermaid">([\s\S]*?)</code></pre>`)

// ConvertMermaidBlocks transforms goldmark mermaid code blocks to mermaid.js format.
func ConvertMermaidBlocks(html string) string {
	return MermaidBlockRe.ReplaceAllString(html, `<pre class="mermaid">$1</pre>`)
}
