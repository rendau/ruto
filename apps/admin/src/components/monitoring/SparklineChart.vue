<script setup lang="ts">
import { computed, ref } from "vue";

const props = withDefaults(
  defineProps<{
    points: Array<{ ts: number; value: number }>;
    color?: string;
    height?: number;
    formatValue?: (value: number) => string;
    formatTs?: (ts: number) => string;
  }>(),
  {
    color: "var(--c-primary)",
    height: 46,
    formatValue: (value: number) => String(value),
    formatTs: (ts: number) =>
      new Date(ts * 1000).toLocaleString(undefined, {
        day: "2-digit",
        month: "2-digit",
        hour: "2-digit",
        minute: "2-digit"
      })
  }
);

const VIEW_W = 200;
const VIEW_H = 40;

const hoverIndex = ref<number | null>(null);

const scaled = computed(() => {
  const points = props.points;
  const first = points[0];
  const last = points[points.length - 1];
  if (!first || !last) return [];
  const maxValue = Math.max(...points.map((p) => p.value), 0);
  const minTs = first.ts;
  const maxTs = last.ts;
  const tsSpan = Math.max(maxTs - minTs, 1);
  // Keep a small headroom on top; a flat zero series draws along the bottom.
  const valueSpan = maxValue > 0 ? maxValue * 1.1 : 1;
  return points.map((p) => ({
    x: ((p.ts - minTs) / tsSpan) * VIEW_W,
    y: VIEW_H - (p.value / valueSpan) * VIEW_H
  }));
});

const linePath = computed(() =>
  scaled.value.map((p, i) => `${i === 0 ? "M" : "L"}${p.x.toFixed(2)},${p.y.toFixed(2)}`).join(" ")
);

const areaPath = computed(() => {
  if (!scaled.value.length) return "";
  return `${linePath.value} L${VIEW_W},${VIEW_H} L0,${VIEW_H} Z`;
});

const hoverPoint = computed(() => {
  if (hoverIndex.value === null) return null;
  const point = props.points[hoverIndex.value];
  const pos = scaled.value[hoverIndex.value];
  if (!point || !pos) return null;
  return { point, leftPct: (pos.x / VIEW_W) * 100, topPct: (pos.y / VIEW_H) * 100 };
});

function onMove(event: MouseEvent): void {
  const first = props.points[0];
  const last = props.points[props.points.length - 1];
  if (!first || !last) return;
  const rect = (event.currentTarget as HTMLElement).getBoundingClientRect();
  const ratio = Math.min(Math.max((event.clientX - rect.left) / rect.width, 0), 1);
  const minTs = first.ts;
  const maxTs = last.ts;
  const targetTs = minTs + ratio * (maxTs - minTs);
  let nearest = 0;
  let nearestDist = Infinity;
  props.points.forEach((p, i) => {
    const dist = Math.abs(p.ts - targetTs);
    if (dist < nearestDist) {
      nearestDist = dist;
      nearest = i;
    }
  });
  hoverIndex.value = nearest;
}

function onLeave(): void {
  hoverIndex.value = null;
}
</script>

<template>
  <div
    class="sparkline"
    :style="{ height: `${height}px` }"
    @mousemove="onMove"
    @mouseleave="onLeave"
  >
    <svg
      v-if="points.length"
      class="sparkline__svg"
      :viewBox="`0 0 ${VIEW_W} ${VIEW_H}`"
      preserveAspectRatio="none"
    >
      <path class="sparkline__area" :d="areaPath" :fill="color" />
      <path
        class="sparkline__line"
        :d="linePath"
        :stroke="color"
        fill="none"
        vector-effect="non-scaling-stroke"
      />
    </svg>
    <span v-else class="sparkline__empty muted">no data</span>

    <template v-if="hoverPoint">
      <span
        class="sparkline__dot"
        :style="{
          left: `${hoverPoint.leftPct}%`,
          top: `${hoverPoint.topPct}%`,
          background: color
        }"
      />
      <span
        class="sparkline__tooltip"
        :class="{ 'sparkline__tooltip--left': hoverPoint.leftPct > 55 }"
        :style="{ left: `${hoverPoint.leftPct}%` }"
      >
        {{ formatValue(hoverPoint.point.value) }}
        <span class="sparkline__tooltip-ts">{{ formatTs(hoverPoint.point.ts) }}</span>
      </span>
    </template>
  </div>
</template>

<style scoped>
.sparkline {
  position: relative;
  width: 100%;
}

.sparkline__svg {
  width: 100%;
  height: 100%;
  display: block;
}

.sparkline__area {
  opacity: 0.14;
}

.sparkline__line {
  stroke-width: 1.5;
}

.sparkline__empty {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100%;
  font-size: 11px;
}

.sparkline__dot {
  position: absolute;
  width: 6px;
  height: 6px;
  border-radius: 50%;
  transform: translate(-50%, -50%);
  pointer-events: none;
}

.sparkline__tooltip {
  position: absolute;
  top: -6px;
  transform: translate(6px, -100%);
  padding: 3px 8px;
  border: 1px solid var(--c-border-strong);
  border-radius: 6px;
  background: var(--c-surface-3);
  color: var(--c-text);
  font-size: 11px;
  font-family: var(--font-mono);
  white-space: nowrap;
  pointer-events: none;
  z-index: 3;
}

.sparkline__tooltip--left {
  transform: translate(calc(-100% - 6px), -100%);
}

.sparkline__tooltip-ts {
  color: var(--c-text-3);
  margin-left: 6px;
}
</style>
