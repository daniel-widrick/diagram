// Browser renderer for diagram layouts.
//
// toSVG(doc, options) turns the render-ready JSON produced by
// `diagram -format json` (package layoutjson) into the same SVG text as the
// Go renderer in render/svg, element for element. mount(container, doc,
// options) puts that SVG in the page and adds hover and selection.
//
// No dependencies, no build step: import it as an ES module.

export const themes = {
  default: {
    background: '#ffffff',
    node: { fill: '#eef1f5', stroke: '#1b2430', text: '#1b2430', textMuted: '#4e5a68' },
    edge: '#1b2430',
    edgeLabel: '#4e5a68',
    barTrack: '#d3dae3',
    barFill: '#1f7a8c',
    groupFill: '#f6f7f9',
    groupStroke: '#9aa3ad',
    kinds: {
      hot: { fill: '#c94a34', stroke: '#c94a34', text: '#ffffff', textMuted: '#ffe1db' },
      warn: { fill: '#f5ebcb', stroke: '#b8860b', text: '#1b2430', textMuted: '#6b5410' },
      weak: { stroke: '#c94a34', strokeDash: '6 4' },
    },
    fontMono: '"Go Mono", "IBM Plex Mono", ui-monospace, Menlo, monospace',
    fontSans: '"Go", "IBM Plex Sans", system-ui, sans-serif',
    styleColors: { muted: 'muted' },
  },
}
themes.tokens = {
  ...themes.default,
  background: 'var(--surface)',
  node: { fill: 'var(--surface-2)', stroke: 'currentColor', text: 'currentColor', textMuted: 'var(--ink-2)' },
  edge: 'currentColor',
  edgeLabel: 'var(--ink-2)',
  barTrack: 'var(--line)',
  barFill: 'var(--accent)',
  groupFill: 'color-mix(in srgb, var(--ink-2) 6%, transparent)',
  groupStroke: 'var(--ink-3)',
  kinds: {
    ...themes.default.kinds,
    hot: { fill: 'var(--hot)', stroke: 'var(--hot)', text: '#ffffff', textMuted: 'rgba(255,255,255,0.85)' },
    weak: { stroke: 'var(--hot)', strokeDash: '6 4' },
  },
}

/** Formats a number rounded to two decimals in its shortest form, like Go's renderer. */
export function fmt(v) {
  const r = Math.round(v * 100) / 100
  if (r === 0) return '0'
  return String(r)
}

function esc(s) {
  return String(s).replace(/&/g, '&amp;').replace(/'/g, '&#39;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&#34;')
}

function fontAttrs(th, st) {
  const family = st && st.family === 'sans' ? th.fontSans : th.fontMono
  const size = st && st.size > 0 ? st.size : 11
  let s = `font-family='${family}' font-size="${fmt(size)}"`
  if (st && st.weight >= 600) s += ` font-weight="${st.weight}"`
  return s
}

function merge(base, over) {
  return {
    fill: over.fill || base.fill,
    stroke: over.stroke || base.stroke,
    text: over.text || base.text,
    textMuted: over.textMuted || base.textMuted,
  }
}

function labelBackground(th) {
  if (th.background === 'transparent' || th.background === 'none') return th.node.fill
  return th.background
}

const cornerRadius = 10

function smoothPath(pts) {
  let d = `M${fmt(pts[0].x)},${fmt(pts[0].y)}`
  for (let i = 1; i + 1 < pts.length; i++) {
    const p = pts[i - 1], c = pts[i], n = pts[i + 1]
    const lin = Math.hypot(c.x - p.x, c.y - p.y)
    const lout = Math.hypot(n.x - c.x, n.y - c.y)
    if (lin < 0.01 || lout < 0.01) continue
    const r = Math.min(cornerRadius, Math.min(lin / 2, lout / 2))
    const a = { x: c.x - (c.x - p.x) / lin * r, y: c.y - (c.y - p.y) / lin * r }
    const b = { x: c.x + (n.x - c.x) / lout * r, y: c.y + (n.y - c.y) / lout * r }
    d += ` L${fmt(a.x)},${fmt(a.y)} Q${fmt(c.x)},${fmt(c.y)} ${fmt(b.x)},${fmt(b.y)}`
  }
  const last = pts[pts.length - 1]
  d += ` L${fmt(last.x)},${fmt(last.y)}`
  return d
}

/**
 * Renders a layout document to SVG text.
 * options: { theme: object|'default'|'tokens', title, description, inline, minStroke, maxStroke }
 */
export function toSVG(doc, options = {}) {
  const th = typeof options.theme === 'string' ? themes[options.theme] : options.theme || themes.default
  const styles = doc.styles || {}
  const style = name => styles[name] || styles[''] || { family: 'mono', size: 11, weight: 0 }
  const minS = options.minStroke > 0 ? options.minStroke : 1
  const maxS = options.maxStroke > 0 ? options.maxStroke : 4
  const W = fmt(doc.size.W), H = fmt(doc.size.H)
  let b = ''
  if (!options.inline) b += '<?xml version="1.0" encoding="UTF-8"?>\n'
  b += `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 ${W} ${H}" width="${W}" height="${H}"`
  if (options.inline) {
    b += ' role="img"'
    if (options.title) b += ` aria-label="${esc(options.title)}"`
  }
  b += '>\n'
  if (options.title) b += `  <title>${esc(options.title)}</title>\n`
  if (options.description) b += `  <desc>${esc(options.description)}</desc>\n`
  b += '  <defs>\n'
  b += `    <marker id="dg-arrow" viewBox="0 0 8 8" refX="7" refY="4" markerWidth="6" markerHeight="6" orient="auto-start-reverse" markerUnits="userSpaceOnUse"><path d="M0,0 L8,4 L0,8 z" fill="${th.edge}"/></marker>\n`
  b += '  </defs>\n'
  if (th.background) b += `  <rect width="${W}" height="${H}" fill="${th.background}"/>\n`

  for (const gr of doc.groups || []) {
    let ks = { fill: th.groupFill, stroke: th.groupStroke, text: th.edgeLabel }
    if (gr.kind && th.kinds[gr.kind]) ks = merge(ks, th.kinds[gr.kind])
    let cls = 'group'
    if (gr.kind) cls += ' ' + esc(gr.kind)
    b += `  <g class="${cls}" data-id="${esc(gr.id)}">\n`
    b += `    <rect x="${fmt(gr.rect.X)}" y="${fmt(gr.rect.Y)}" width="${fmt(gr.rect.W)}" height="${fmt(gr.rect.H)}" rx="8" fill="${ks.fill}" stroke="${ks.stroke}" stroke-dasharray="4 3"/>\n`
    if (gr.label) {
      b += `    <text x="${fmt(gr.labelX)}" y="${fmt(gr.labelY)}" fill="${ks.text}" ${fontAttrs(th, style('title'))} xml:space="preserve">${esc(gr.label)}</text>\n`
    }
    b += '  </g>\n'
  }

  for (const e of doc.edges || []) {
    let stroke = th.edge, dash = ''
    if (e.kind && th.kinds[e.kind]) {
      if (th.kinds[e.kind].stroke) stroke = th.kinds[e.kind].stroke
      dash = th.kinds[e.kind].strokeDash || ''
    }
    const weight = Math.max(0, Math.min(1, e.weight || 0))
    const w = minS + weight * (maxS - minS)
    const pts = e.path.map(p => ({ x: p.X, y: p.Y }))
    if (e.curved && pts.length > 2) {
      b += `  <path d="${smoothPath(pts)}" fill="none" stroke="${stroke}" stroke-width="${fmt(w)}" stroke-linejoin="round"`
    } else {
      b += `  <polyline points="${pts.map(p => `${fmt(p.x)},${fmt(p.y)}`).join(' ')}" fill="none" stroke="${stroke}" stroke-width="${fmt(w)}" stroke-linejoin="round"`
    }
    if (dash) b += ` stroke-dasharray="${dash}"`
    if (e.arrow === 'forward') b += ' marker-end="url(#dg-arrow)"'
    else if (e.arrow === 'backward') b += ' marker-start="url(#dg-arrow)"'
    else if (e.arrow === 'both') b += ' marker-start="url(#dg-arrow)" marker-end="url(#dg-arrow)"'
    b += e.kind ? ` class="edge ${esc(e.kind)}"` : ' class="edge"'
    b += '/>\n'
    if (e.label) {
      const lb = e.label
      if (lb.background && th.background) {
        b += `  <rect x="${fmt(lb.box.X)}" y="${fmt(lb.box.Y)}" width="${fmt(lb.box.W)}" height="${fmt(lb.box.H)}" rx="3" fill="${labelBackground(th)}"/>\n`
      }
      b += `  <text x="${fmt(lb.X)}" y="${fmt(lb.Y)}" text-anchor="${lb.anchor}" fill="${th.edgeLabel}" ${fontAttrs(th, style(lb.style))} xml:space="preserve">${esc(lb.text)}</text>\n`
    }
  }

  for (const n of doc.nodes || []) {
    let ks = th.node
    if (n.kind && th.kinds[n.kind]) ks = merge(th.node, th.kinds[n.kind])
    let cls = 'node'
    if (n.kind) cls += ' ' + esc(n.kind)
    b += `  <g class="${cls}" data-id="${esc(n.id)}">\n`
    b += `    <rect x="${fmt(n.rect.X)}" y="${fmt(n.rect.Y)}" width="${fmt(n.rect.W)}" height="${fmt(n.rect.H)}" rx="4" fill="${ks.fill}" stroke="${ks.stroke}"/>\n`
    for (const ln of n.lines || []) {
      for (const sp of ln.spans || []) {
        if (!sp.text) continue
        const color = th.styleColors && th.styleColors[sp.style] === 'muted' ? ks.textMuted : ks.text
        const title = sp.full !== sp.text ? `<title>${esc(sp.full)}</title>` : ''
        b += `    <text x="${fmt(sp.x)}" y="${fmt(ln.y)}" fill="${color}" ${fontAttrs(th, style(sp.style))} xml:space="preserve">${title}${esc(sp.text)}</text>\n`
      }
    }
    if (n.badge) {
      const bd = n.badge
      b += `    <rect x="${fmt(bd.X)}" y="${fmt(bd.Y)}" width="${fmt(bd.W)}" height="16" rx="8" fill="${ks.stroke}" class="badge"/>\n`
      b += `    <text x="${fmt(bd.X + bd.W / 2)}" y="${fmt(bd.textY)}" text-anchor="middle" fill="${th.background}" ${fontAttrs(th, style('edge'))}>${bd.text}</text>\n`
    }
    if (n.bar) {
      const r = n.bar
      b += `    <rect x="${fmt(r.X)}" y="${fmt(r.Y)}" width="${fmt(r.W)}" height="${fmt(r.H)}" rx="2" fill="${th.barTrack}"/>\n`
      const fill = n.kind === 'hot' ? ks.text : th.barFill
      if (r.fill > 0) b += `    <rect x="${fmt(r.X)}" y="${fmt(r.Y)}" width="${fmt(r.fill)}" height="${fmt(r.H)}" rx="2" fill="${fill}"/>\n`
    }
    b += '  </g>\n'
  }
  b += '</svg>\n'
  return b
}

/**
 * Mounts the diagram into a container element with hover and selection.
 * options: toSVG options plus onSelect(id|null, node|null), onHover(id|null).
 * Returns { svg, select(id), selected(), destroy() }.
 */
export function mount(container, doc, options = {}) {
  container.innerHTML = toSVG(doc, { ...options, inline: true })
  const svg = container.querySelector('svg')
  const byId = new Map((doc.nodes || []).map(n => [n.id, n]))
  let selectedId = null
  const nodeOf = target => target && target.closest ? target.closest('g.node[data-id]') : null
  const setSelected = id => {
    selectedId = id
    for (const g of svg.querySelectorAll('g.node')) g.classList.toggle('selected', g.dataset.id === id)
    if (options.onSelect) options.onSelect(id, id === null ? null : byId.get(id) || null)
  }
  const onClick = e => {
    const g = nodeOf(e.target)
    setSelected(g ? (g.dataset.id === selectedId ? null : g.dataset.id) : null)
  }
  const onOver = e => {
    const g = nodeOf(e.target)
    for (const el of svg.querySelectorAll('g.node.hover')) if (el !== g) el.classList.remove('hover')
    if (g) g.classList.add('hover')
    if (options.onHover) options.onHover(g ? g.dataset.id : null)
  }
  const onOut = e => {
    const g = nodeOf(e.target)
    if (g && !g.contains(e.relatedTarget)) {
      g.classList.remove('hover')
      if (options.onHover) options.onHover(null)
    }
  }
  const onKey = e => { if (e.key === 'Escape') setSelected(null) }
  svg.addEventListener('click', onClick)
  svg.addEventListener('mouseover', onOver)
  svg.addEventListener('mouseout', onOut)
  container.addEventListener('keydown', onKey)
  if (!container.hasAttribute('tabindex')) container.setAttribute('tabindex', '0')
  return {
    svg,
    select: setSelected,
    selected: () => selectedId,
    destroy() {
      svg.removeEventListener('click', onClick)
      svg.removeEventListener('mouseover', onOver)
      svg.removeEventListener('mouseout', onOut)
      container.removeEventListener('keydown', onKey)
      container.innerHTML = ''
    },
  }
}
