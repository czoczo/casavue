<template>
  <div id="error-box-container">
    <div id="error-box-text">
      <div id="error-text-main">
        ¯\_(ツ)_/¯
      </div>
      <div id="error-text-msg">
        Is CasaVue server available?
      </div>
    </div>
  </div>
</template>

<script>
</script>

<style>

#error-box-container {
  position: absolute;
  top: 60%;
  left: 50%;
  transform: translate(-50%, -50%);
  padding: 3em;
  text-align: center;
  display: flex;
  justify-content: center;
  width: 400px;
  color: var(--theme-color-40);
  /*border: 4px dashed var(--theme-color);
  border-radius: 30px;*/
}

#error-box-text {
  display: inline-block;
  vertical-align: middle;
  line-height: 1.5;
}

#error-text-main {
  font-size: 3em;
  font-weight: bold;
  margin-bottom: 0.3em;
}

@media (max-width: 480px) {
  #error-box-container {
    width: 95%;
  }
}
</style>
