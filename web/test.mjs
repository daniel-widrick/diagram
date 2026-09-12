// Compares the browser renderer with the Go renderer for every example:
// the SVG text must be identical. Run from the repository root:
//   node web/test.mjs
import { execFileSync } from 'node:child_process'
import { readFileSync } from 'node:fs'
import { toSVG } from './diagram.js'

const cases = [
  { file: 'testdata/plan.json', layout: 'tree', title: 'Plan tree' },
  { file: 'testdata/collapsed.json', layout: 'tree', title: 'Collapsed' },
  { file: 'testdata/join.json', layout: 'layered', title: 'Join graph' },
  { file: 'testdata/groups.json', layout: 'layered', title: 'Groups' },
]
let failed = 0
for (const c of cases) {
  for (const theme of ['default', 'tokens']) {
    for (const inline of [false, true]) {
      const input = readFileSync(c.file)
      const args = ['run', './cmd/diagram', '-layout', c.layout, '-theme', theme, '-title', c.title]
      if (inline) args.push('-inline')
      const goSvg = execFileSync('go', args, { input }).toString()
      const doc = JSON.parse(execFileSync('go', ['run', './cmd/diagram', '-layout', c.layout, '-format', 'json'], { input }).toString())
      const jsSvg = toSVG(doc, { theme, title: c.title, inline })
      if (goSvg === jsSvg) {
        console.log(`ok   ${c.file} ${theme}${inline ? ' inline' : ''}`)
      } else {
        failed++
        const a = goSvg.split('\n'), b = jsSvg.split('\n')
        let i = 0
        while (i < a.length && a[i] === b[i]) i++
        console.log(`FAIL ${c.file} ${theme}${inline ? ' inline' : ''}: first difference at line ${i + 1}\n  go: ${a[i]}\n  js: ${b[i]}`)
      }
    }
  }
}
if (failed) { console.log(`${failed} mismatches`); process.exit(1) }
console.log('browser renderer matches the Go renderer')
