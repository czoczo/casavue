<template>
  <div id="top">
    <div id="topLogo">
    <TopLogo />
    </div>
    <div id="title">
      {{ config.customization.name }}
    </div>
    <div id="iconsTopRight">
      <font-awesome-icon icon="fa-solid fa-brush" @click="$emit('show-settings', true)" />
    </div>
  </div>
</template>

<script>
import TopLogo from './TopLogo.vue'
export default {
  components: {
    TopLogo,
  },
}
</script>

<style>
#top {
  z-index: 999;
  display: flex;
  background-color: var(--theme-color);
  height: 70px;
  align-items: stretch;
}

#topLogo {
  width: 90px;
}

#topLogo svg {
  max-height: 70px;
  padding: 10px 6px 8px 8px;
}

#top svg path {
  fill: var(--theme-color-contrast);
}

#title {
  padding-top: 16px;
  color: var(--theme-color-contrast);
  font-family: 'Archivo Black', sans-serif;
  font-size: 48px;
  width: 100%;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

#iconsTopRight {
  padding: 10px 10px 0px 0px;
  font-size: 1.4rem;
}
@media (max-width: 900px) {
  #title {
    font-size: 32px;
    padding: 24px 0px 0px 8px;
  }
}
</style>
