import * as THREE from 'three'
import { GLTFLoader } from 'three/addons/loaders/GLTFLoader.js'
import { OrbitControls } from 'three/addons/controls/OrbitControls.js'

// The page: a sprite in, its model shown turning and offered as a .glb. It converts with
// sprite2model.wasm (the GitHub page) or, run by `sprite2model serve`, through its /api/convert.

const EXAMPLES = ['pillar', 'crate', 'barrel', 'tree', 'wall', 'golem_outfit']
const READING_NAMES = { box: 'Box', round: 'Round', wall: 'Wall', cutout: 'Cut-out', hull: 'Hull', outfit: 'Outfit sheet' }
const $ = (id) => document.getElementById(id)

let sprite = null // { name, bytes, url }
let result = null
let seq = 0

// The converter: the local server's, else WebAssembly.
const engine = (async () => {
	try {
		const r = await fetch('api/ping')
		if (r.ok && (r.headers.get('content-type') ?? '').includes('json')) {
			const { version } = await r.json()
			return { name: 'local', version, convert: convertLocal }
		}
	} catch {}
	await new Promise((ok, fail) => {
		const s = document.createElement('script')
		s.src = 'wasm_exec.js'
		s.onload = ok
		s.onerror = () => fail(new Error('wasm_exec.js did not load'))
		document.head.append(s)
	})
	const go = new globalThis.Go()
	const { instance } = await WebAssembly.instantiateStreaming(fetch('sprite2model.wasm'), go.importObject)
	go.run(instance)
	return { name: 'wasm', version: globalThis.sprite2model.version, convert: convertWasm }
})()

async function convertLocal(bytes, options, name) {
	const q = new URLSearchParams({ name, options: JSON.stringify(options) })
	const r = await fetch(`api/convert?${q}`, { method: 'POST', body: bytes })
	return r.json()
}

async function convertWasm(bytes, options, name) {
	return JSON.parse(globalThis.sprite2model.convert(bytes, JSON.stringify(options), name))
}

// The 3D view.
const canvas = $('view')
const renderer = new THREE.WebGLRenderer({ canvas, antialias: true })
renderer.setPixelRatio(window.devicePixelRatio)
const scene = new THREE.Scene()
scene.background = new THREE.Color(0x1d1d22)
const camera = new THREE.PerspectiveCamera(32, 1, 0.01, 100)
scene.add(new THREE.HemisphereLight(0xdfe6ff, 0x302820, 1.4))
const sun = new THREE.DirectionalLight(0xffffff, 2.2)
sun.position.set(-2, 4, -1.5)
scene.add(sun)
const grid = new THREE.GridHelper(5, 5, 0x75757f, 0x3a3a42)
scene.add(grid)
const controls = new OrbitControls(camera, canvas)
controls.enableDamping = true
controls.autoRotate = true
controls.autoRotateSpeed = 1.5
controls.target.set(0, 0.5, 0)
camera.position.set(2.2, 2.2, 2.2)
let model = null
let radius = 1
const maps = new Map()

function fit() {
	const w = canvas.clientWidth
	const h = canvas.clientHeight
	renderer.setSize(w, h, false)
	camera.aspect = w / Math.max(1, h)
	camera.updateProjectionMatrix()
}
new ResizeObserver(fit).observe(canvas)
fit()
renderer.setAnimationLoop(() => {
	controls.update()
	renderer.render(scene, camera)
})

function toggle(id, on) {
	const b = $(id)
	b.classList.toggle('on', on)
	b.setAttribute('aria-pressed', String(on))
}
const isOn = (id) => $(id).classList.contains('on')

$('turn').onclick = () => {
	toggle('turn', !isOn('turn'))
	controls.autoRotate = isOn('turn')
}
controls.addEventListener('start', () => {
	toggle('turn', false)
	controls.autoRotate = false
})
$('wire').onclick = () => {
	toggle('wire', !isOn('wire'))
	looks()
}
$('texture').onclick = () => {
	toggle('texture', !isOn('texture'))
	looks()
}
$('grid').onclick = () => {
	toggle('grid', !isOn('grid'))
	grid.visible = isOn('grid')
}
for (const b of document.querySelectorAll('[data-view]')) b.onclick = () => setView(b.dataset.view)

function looks() {
	for (const [mat, map] of maps) {
		mat.wireframe = isOn('wire')
		mat.map = isOn('texture') ? map : null
		mat.needsUpdate = true
	}
}

function setView(view, target = controls.target.clone()) {
	const dir = {
		drawn: new THREE.Vector3(1, 1, 1),
		front: new THREE.Vector3(0, 0.15, 1),
		side: new THREE.Vector3(1, 0.15, 0),
		top: new THREE.Vector3(0.001, 1, 0.001),
	}[view].normalize()
	controls.target.copy(target)
	camera.position.copy(target).addScaledVector(dir, radius * 4)
	controls.update()
}

function dropModel() {
	if (!model) return
	scene.remove(model)
	model.traverse((o) => {
		if (!o.isMesh) return
		o.geometry.dispose()
		maps.get(o.material)?.dispose()
		o.material.dispose()
	})
	maps.clear()
	model = null
}

const b64bytes = (s) => Uint8Array.from(atob(s), (c) => c.charCodeAt(0))

function showModel(glb, keepView) {
	new GLTFLoader().parse(b64bytes(glb).buffer, '', (gltf) => {
		dropModel()
		model = gltf.scene
		model.traverse((o) => {
			if (o.isMesh) maps.set(o.material, o.material.map)
		})
		looks()
		scene.add(model)
		const box = new THREE.Box3().setFromObject(model)
		radius = Math.max(box.getSize(new THREE.Vector3()).length() / 2, 0.3)
		if (!keepView) setView('drawn', box.getCenter(new THREE.Vector3()))
	})
}

// Options.
function options() {
	const o = { projection: $('projection').value, reading: $('reading').value, keepOutline: $('outline').checked, frame: Number($('frame').value || 0) }
	if ($('centre').checked) o.centre = { X: Number($('cx').value), Y: Number($('cy').value) }
	return o
}
for (const id of ['projection', 'reading', 'outline', 'frame', 'cx', 'cy']) $(id).addEventListener('change', () => convert(true))
$('centre').addEventListener('change', () => {
	$('centreRow').hidden = !$('centre').checked
	if ($('centre').checked && result) {
		$('cx').value = result.report.centre.X
		$('cy').value = result.report.centre.Y
	}
	convert(true)
})

async function convert(keepView) {
	if (!sprite) return
	const mine = ++seq
	$('busy').hidden = false
	$('error').hidden = true
	try {
		const conv = await engine
		const r = await conv.convert(sprite.bytes, options(), sprite.name)
		if (mine !== seq) return
		if (r.error) throw new Error(r.error)
		result = r
		showModel(r.glb, keepView)
		report(r)
	} catch (err) {
		if (mine !== seq) return
		result = null
		dropModel()
		$('error').textContent = String(err.message ?? err)
		$('error').hidden = false
		$('stats').replaceChildren()
		$('download').disabled = true
	} finally {
		if (mine === seq) $('busy').hidden = true
	}
}

function report(r) {
	const rep = r.report
	$('sides').src = `data:image/png;base64,${r.sides}`
	$('sidesCaption').textContent = rep.reading === 'outfit' ? 'The model drawn facing south, west, north and east' : 'The model drawn the same way, from its four sides'
	const rows = [
		['Read as', `${READING_NAMES[rep.reading] ?? rep.reading}${rep.run ? ` (${rep.run})` : ''}`],
		['Centre', `${rep.centre.X}, ${rep.centre.Y}`],
		['Sprite', `${rep.width} x ${rep.height}`],
		['Size', `${rep.size.map((n) => n.toFixed(2)).join(' x ')} tiles`],
		['Triangles', rep.triangles.toLocaleString()],
		['Voxels', rep.voxels.toLocaleString()],
		['Explains', `${rep.covered}% of the sprite`],
		['Plates', `${rep.extra.toLocaleString()} voxels`],
		['File', `${(atob(r.glb).length / 1024).toFixed(1)} KB`],
	]
	if (rep.scores?.length > 1) rows.push(['Tried', rep.scores.map((s) => `${READING_NAMES[s.reading] ?? s.reading} ${s.covered}%`).join('\n')])
	$('stats').replaceChildren(
		...rows.flatMap(([k, v]) => {
			const dt = document.createElement('dt')
			dt.textContent = k
			const dd = document.createElement('dd')
			dd.textContent = v
			return [dt, dd]
		}),
	)
	// The frame picker, for an outfit sheet.
	const frames = r.frames ?? 0
	$('frameRow').hidden = frames < 2
	const sel = $('frame')
	if (sel.options.length !== frames) {
		const at = Number(sel.value || 0)
		sel.replaceChildren(
			...Array.from({ length: frames }, (_, i) => {
				const o = document.createElement('option')
				o.value = String(i)
				o.textContent = `${i + 1} of ${frames}`
				return o
			}),
		)
		sel.value = String(Math.min(at, Math.max(0, frames - 1)))
	}
	$('download').disabled = false
}

$('download').onclick = () => {
	if (!result) return
	const blob = new Blob([b64bytes(result.glb)], { type: 'model/gltf-binary' })
	const a = document.createElement('a')
	a.href = URL.createObjectURL(blob)
	a.download = `${result.name}.glb`
	a.click()
	setTimeout(() => URL.revokeObjectURL(a.href), 1000)
}

// Picking a sprite.
async function load(name, blob) {
	if (sprite?.url) URL.revokeObjectURL(sprite.url)
	const bytes = new Uint8Array(await blob.arrayBuffer())
	sprite = { name: name.replace(/\.[^.]+$/, ''), bytes, url: URL.createObjectURL(blob) }
	$('sprite').src = sprite.url
	$('name').textContent = name
	$('hint').hidden = true
	$('frame').replaceChildren()
	$('centre').checked = false
	$('centreRow').hidden = true
	for (const t of document.querySelectorAll('.examples button')) t.classList.toggle('on', t.dataset.name === sprite.name)
	await convert(false)
}

$('file').addEventListener('change', () => {
	const f = $('file').files?.[0]
	if (f) load(f.name, f)
})
const drop = $('drop')
drop.addEventListener('keydown', (e) => (e.key === 'Enter' || e.key === ' ') && $('file').click())
for (const ev of ['dragenter', 'dragover']) drop.addEventListener(ev, (e) => (e.preventDefault(), drop.classList.add('over')))
for (const ev of ['dragleave', 'drop']) drop.addEventListener(ev, () => drop.classList.remove('over'))
drop.addEventListener('drop', (e) => {
	e.preventDefault()
	const f = e.dataTransfer?.files?.[0]
	if (f) load(f.name, f)
})
// A dropped file anywhere on the page.
window.addEventListener('dragover', (e) => e.preventDefault())
window.addEventListener('drop', (e) => {
	e.preventDefault()
	const f = e.dataTransfer?.files?.[0]
	if (f) load(f.name, f)
})

async function loadExample(name) {
	const r = await fetch(`examples/${name}.png`)
	if (r.ok) await load(`${name}.png`, await r.blob())
}
for (const name of EXAMPLES) {
	const b = document.createElement('button')
	b.dataset.name = name
	b.title = name.replace('_', ' ')
	const img = document.createElement('img')
	img.src = `examples/${name}.png`
	img.alt = name
	b.append(img)
	b.onclick = () => loadExample(name)
	$('examples').append(b)
}

engine.then(
	(e) => {
		$('version').textContent = e.version
		$('engine').textContent = e.name === 'wasm' ? 'Converted in your browser (WebAssembly): your sprites never leave your computer.' : 'Converted by the local sprite2model.'
	},
	(err) => {
		$('error').textContent = `The converter did not load: ${err.message ?? err}`
		$('error').hidden = false
	},
)

const first = new URLSearchParams(location.search).get('example')
if (first) loadExample(first)
