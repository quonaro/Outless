<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'

const canvasRef = ref<HTMLCanvasElement | null>(null)

interface Node {
  x: number
  y: number
  vx: number
  vy: number
}

const NODE_COUNT = 40
const CONNECTION_DISTANCE = 180
const NODE_COLOR = 'rgba(255, 255, 255, 0.5)'
const MOUSE_CONNECTION_DISTANCE = 200

let animationId: number | null = null
let nodes: Node[] = []
let width = 0
let height = 0
let mouseX = -1000
let mouseY = -1000

function resize() {
  const canvas = canvasRef.value
  if (!canvas) return
  const parent = canvas.parentElement
  if (!parent) return
  width = parent.clientWidth
  height = parent.clientHeight
  const dpr = window.devicePixelRatio || 1
  canvas.width = width * dpr
  canvas.height = height * dpr
  canvas.style.width = `${width}px`
  canvas.style.height = `${height}px`
  const ctx = canvas.getContext('2d')
  if (ctx) {
    ctx.scale(dpr, dpr)
  }
}

function initNodes() {
  nodes = []
  for (let i = 0; i < NODE_COUNT; i++) {
    nodes.push({
      x: Math.random() * width,
      y: Math.random() * height,
      vx: (Math.random() - 0.5) * 0.5,
      vy: (Math.random() - 0.5) * 0.5,
    })
  }
}

function draw() {
  const canvas = canvasRef.value
  if (!canvas) return
  const ctx = canvas.getContext('2d')
  if (!ctx) return

  ctx.clearRect(0, 0, width, height)

  for (let i = 0; i < nodes.length; i++) {
    const a = nodes[i]!

    // Move node
    a.x += a.vx
    a.y += a.vy

    // Bounce off walls
    if (a.x < 0 || a.x > width) a.vx *= -1
    if (a.y < 0 || a.y > height) a.vy *= -1

    // Clamp to bounds
    a.x = Math.max(0, Math.min(width, a.x))
    a.y = Math.max(0, Math.min(height, a.y))

    // Draw connections to other nodes
    for (let j = i + 1; j < nodes.length; j++) {
      const b = nodes[j]!
      const dx = a.x - b.x
      const dy = a.y - b.y
      const dist = Math.sqrt(dx * dx + dy * dy)
      if (dist < CONNECTION_DISTANCE) {
        const alpha = 1 - dist / CONNECTION_DISTANCE
        ctx.beginPath()
        ctx.moveTo(a.x, a.y)
        ctx.lineTo(b.x, b.y)
        ctx.strokeStyle = `rgba(255, 255, 255, ${alpha * 0.3})`
        ctx.lineWidth = 1
        ctx.stroke()
      }
    }

    // Connection to mouse
    const mdx = a.x - mouseX
    const mdy = a.y - mouseY
    const mDist = Math.sqrt(mdx * mdx + mdy * mdy)
    if (mDist < MOUSE_CONNECTION_DISTANCE) {
      const alpha = 1 - mDist / MOUSE_CONNECTION_DISTANCE
      ctx.beginPath()
      ctx.moveTo(a.x, a.y)
      ctx.lineTo(mouseX, mouseY)
      ctx.strokeStyle = `rgba(255, 255, 255, ${alpha * 0.5})`
      ctx.lineWidth = 1
      ctx.stroke()
    }
  }

  // Draw nodes
  for (const node of nodes) {
    ctx.beginPath()
    ctx.arc(node.x, node.y, 2, 0, Math.PI * 2)
    ctx.fillStyle = NODE_COLOR
    ctx.fill()
  }

  animationId = requestAnimationFrame(draw)
}

function onMouseMove(e: MouseEvent) {
  const canvas = canvasRef.value
  if (!canvas) return
  const rect = canvas.getBoundingClientRect()
  mouseX = e.clientX - rect.left
  mouseY = e.clientY - rect.top
}

function onMouseLeave() {
  mouseX = -1000
  mouseY = -1000
}

onMounted(() => {
  resize()
  initNodes()
  draw()
  window.addEventListener('resize', () => {
    resize()
    initNodes()
  })
})

onUnmounted(() => {
  if (animationId) cancelAnimationFrame(animationId)
})
</script>

<template>
  <canvas
    ref="canvasRef"
    class="absolute inset-0 w-full h-full"
    @mousemove="onMouseMove"
    @mouseleave="onMouseLeave"
  />
</template>
