<template>
  <footer>
    <a href="https://casavue.app">CasaVue</a> ver. {{ version }}
  </footer>
</template>

<script>
export default {
  data() {
    return {
      version: 'a.b.c'
    };
  },
  mounted() {
    this.version = this.config.version;
  },
}
</script>

<style>
footer {
  font-size: 0.8rem;
  flex-shrink: 0;
  text-align: center;
  border-top: 1px var(--theme-color-20) solid;
  padding: 10px 0px;
  color: var(--base-color-contrast);
}

footer svg {
  margin: 0px 0px 0px 5px;
  stroke: var(--theme-color-contrast);
  stroke-width: 25;
}
</style>
