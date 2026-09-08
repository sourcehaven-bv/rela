#!/usr/bin/env python3
"""Render a built manual as a self-contained HTML page for visual inspection.

Two things pandoc will not do on its own:
  - mermaid fences are code blocks to it, so they render as source. They are
    rewritten to <pre class="mermaid"> and mermaid.js renders them client-side.
  - the figures are referenced by path; inlining them as data: URIs makes the
    file portable, which matters because the point is to SHOW someone.
"""
import base64, mimetypes, pathlib, re, subprocess, sys

src = pathlib.Path(sys.argv[1])
out = pathlib.Path(sys.argv[2])
md = src.read_text()

# Mermaid fences -> a marker pandoc passes through untouched, restored below.
blocks = []
def stash(m):
    blocks.append(m.group(1))
    return f"\n\nMERMAIDBLOCK{len(blocks)-1}ENDMARK\n\n"
md = re.sub(r"```mermaid\n(.*?)```", stash, md, flags=re.S)

html = subprocess.run(
    ["pandoc", "--from=gfm", "--to=html5", "--standalone", "--metadata", "title=Worlds"],
    input=md, capture_output=True, text=True, check=True,
).stdout

for i, b in enumerate(blocks):
    marker = f"MERMAIDBLOCK{i}ENDMARK"
    pre = '<pre class="mermaid">' + b.replace("&", "&amp;").replace("<", "&lt;") + "</pre>"
    html = re.sub(rf"<p>\s*{marker}\s*</p>", pre, html)
    html = html.replace(marker, pre)

# Inline the figures so the file travels on its own.
def inline(m):
    p = src.parent / m.group(1)
    if not p.exists():
        return m.group(0)
    mime = mimetypes.guess_type(p.name)[0] or "image/png"
    b64 = base64.b64encode(p.read_bytes()).decode()
    return f'src="data:{mime};base64,{b64}"'
html = re.sub(r'src="([^"]+)"', inline, html)

STYLE = """
<style>
  :root { --ink:#1a1d21; --muted:#5b6570; --rule:#e3e6ea; --accent:#1f6feb; --bg:#fbfbfc; }
  html { box-sizing: border-box; }
  *, *::before, *::after { box-sizing: inherit; }
  body {
    margin: 0 auto; padding: 3rem 1.5rem 6rem; max-width: 46rem;
    font: 16px/1.65 -apple-system, BlinkMacSystemFont, "Segoe UI", system-ui, sans-serif;
    color: var(--ink); background: var(--bg);
  }
  h1 { font-size: 2.1rem; line-height: 1.2; margin: 0 0 .4em; letter-spacing: -.02em; }
  h2 { font-size: 1.4rem; margin: 2.6em 0 .6em; padding-top: 1.2em;
       border-top: 1px solid var(--rule); letter-spacing: -.01em; }
  h3 { font-size: 1.1rem; margin: 2em 0 .5em; color: var(--muted);
       text-transform: uppercase; letter-spacing: .06em; }
  p, li { color: var(--ink); }
  code { font: .875em/1.5 ui-monospace, SFMono-Regular, "SF Mono", Menlo, monospace;
         background: #eef1f4; padding: .15em .4em; border-radius: 4px; }
  pre { background: #f4f6f8; border: 1px solid var(--rule); border-radius: 8px;
        padding: 1rem 1.1rem; overflow-x: auto; }
  pre code { background: none; padding: 0; font-size: .85rem; }
  table { border-collapse: collapse; width: 100%; margin: 1.4em 0; font-size: .93rem; }
  th, td { text-align: left; padding: .55rem .7rem; border-bottom: 1px solid var(--rule); }
  th { font-size: .78rem; text-transform: uppercase; letter-spacing: .05em; color: var(--muted); }
  tr:last-child td { border-bottom: none; }
  /* Figures: a screenshot is evidence, so give it a frame and room. */
  img { max-width: 100%; height: auto; display: block; margin: 1.6em auto;
        border: 1px solid var(--rule); border-radius: 10px;
        box-shadow: 0 2px 14px rgba(20,30,45,.09); }
  .mermaid { background: #fff; border: 1px solid var(--rule); border-radius: 10px;
             padding: 1.2rem; text-align: center; overflow-x: auto; }
  blockquote { margin: 1.4em 0; padding: .2em 1.1em; border-left: 3px solid var(--accent);
               color: var(--muted); }
  a { color: var(--accent); }
  @media (prefers-color-scheme: dark) {
    :root { --ink:#e6e9ee; --muted:#9aa4b1; --rule:#2b3138; --bg:#15181c; }
    code { background:#22272e; } pre { background:#1b1f24; }
    .mermaid { background:#1b1f24; }
    img { box-shadow: 0 2px 14px rgba(0,0,0,.4); }
  }
</style>
<script type="module">
  import mermaid from 'https://cdn.jsdelivr.net/npm/mermaid@11/dist/mermaid.esm.min.mjs';
  const dark = window.matchMedia('(prefers-color-scheme: dark)').matches;
  // startOnLoad:false + an explicit per-block render.
  //
  // Every diagram here is generated independently and numbers its nodes from
  // n0, so the ids repeat across blocks. Auto-render resolves them in one
  // shared namespace: the second diagram drew every graph on top of itself and
  // the third came out empty. Rendering each block under its own unique id
  // keeps them separate.
  mermaid.initialize({ startOnLoad: false, theme: dark ? 'dark' : 'default' });
  const blocks = document.querySelectorAll('pre.mermaid');
  for (let i = 0; i < blocks.length; i++) {
    const el = blocks[i];
    const { svg } = await mermaid.render('mmd' + i, el.textContent.trim());
    const box = document.createElement('div');
    box.className = 'mermaid';
    box.innerHTML = svg;
    el.replaceWith(box);
  }
</script>
"""
html = html.replace("</head>", STYLE + "</head>")
out.write_text(html)
print(f"✓ {out}  ({len(blocks)} diagrams, {html.count('data:image')} inlined figures)")
