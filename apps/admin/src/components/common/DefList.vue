<template>
  <div class="def-list"><slot /></div>
</template>

<style scoped>
.def-list {
  display: flex;
  flex-direction: column;
  border: 1px solid var(--c-border);
  border-radius: 10px;
  overflow: hidden;
}
</style>
