<!-- PWAUpdatePrompt.vue -->
<template>
  <div class="pwa-update-prompt" v-if="updateExists">
    An update is available
    <button @click="refreshApp">
      Update
    </button>
  </div>
</template>

<script>
import update from '../mixins/update'

export default {
  mixins: [update],
}
</script>

<style>
.pwa-update-prompt {
  position: fixed;
  padding: 16px 0px;
  margin-bottom: 20px;
  left: 0;
  bottom: 0;
  width: 100%;
  color: var(--theme-color-lowest);
  text-align: center;
  background-color: var(--base-color-light);
  -webkit-transition: background 1s;
  transition: background 1s;
  box-shadow: rgba(60, 64, 67, 0.3) 0px 1px 2px 0px, rgba(60, 64, 67, 0.15) 0px 2px 6px 2px;
}
</style>
