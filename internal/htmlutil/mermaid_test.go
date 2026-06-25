package htmlutil

import "testing"

func TestConvertMermaidBlocks(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "basic mermaid block",
			input:    `<pre><code class="language-mermaid">graph TD; A--&gt;B;</code></pre>`,
			expected: `<pre class="mermaid">graph TD; A--&gt;B;</pre>`,
		},
		{
			name:     "multiple mermaid blocks",
			input:    `<pre><code class="language-mermaid">A</code></pre><p>text</p><pre><code class="language-mermaid">B</code></pre>`,
			expected: `<pre class="mermaid">A</pre><p>text</p><pre class="mermaid">B</pre>`,
		},
		{
			name:     "multiline mermaid",
			input:    "<pre><code class=\"language-mermaid\">graph TD\n    A-->B\n    B-->C</code></pre>",
			expected: "<pre class=\"mermaid\">graph TD\n    A-->B\n    B-->C</pre>",
		},
		{
			name:     "no mermaid blocks",
			input:    `<pre><code class="language-go">fmt.Println("hello")</code></pre>`,
			expected: `<pre><code class="language-go">fmt.Println("hello")</code></pre>`,
		},
		{
			name:     "empty input",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertMermaidBlocks(tt.input)
			if result != tt.expected {
				t.Errorf("ConvertMermaidBlocks() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestConvertPlantUMLBlocks(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "basic plantuml block",
			input:    `<pre><code class="language-plantuml">@startuml A--&gt;B @enduml</code></pre>`,
			expected: `<pre class="plantuml">@startuml A--&gt;B @enduml</pre>`,
		},
		{
			name:     "multiline plantuml",
			input:    "<pre><code class=\"language-plantuml\">@startuml\nA->B\n@enduml</code></pre>",
			expected: "<pre class=\"plantuml\">@startuml\nA->B\n@enduml</pre>",
		},
		{
			name:     "leaves mermaid untouched",
			input:    `<pre><code class="language-mermaid">graph TD</code></pre>`,
			expected: `<pre><code class="language-mermaid">graph TD</code></pre>`,
		},
		{
			name:     "no plantuml blocks",
			input:    `<pre><code class="language-go">x</code></pre>`,
			expected: `<pre><code class="language-go">x</code></pre>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertPlantUMLBlocks(tt.input)
			if result != tt.expected {
				t.Errorf("ConvertPlantUMLBlocks() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestConvertDiagramBlocks(t *testing.T) {
	// Both rewrites apply in a single pass; an unrelated code block is untouched.
	input := `<pre><code class="language-mermaid">graph TD</code></pre>` +
		`<pre><code class="language-plantuml">@startuml</code></pre>` +
		`<pre><code class="language-go">x</code></pre>`
	want := `<pre class="mermaid">graph TD</pre>` +
		`<pre class="plantuml">@startuml</pre>` +
		`<pre><code class="language-go">x</code></pre>`
	if got := ConvertDiagramBlocks(input); got != want {
		t.Errorf("ConvertDiagramBlocks() = %q, want %q", got, want)
	}
}
