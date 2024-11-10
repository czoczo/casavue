<template>
  <div id="container">
    <div id="content">
      <TopArea @show-settings="showSettings" />
      <transition name="fade">
        <ThemeSettings :settingsVisible="settingsVisible" @show-settings="showSettings" v-if="settingsVisible" />
      </transition>
      <DashBoard />
      <PWAInstallPrompt />
      <PWAUpdatePrompt />
    </div>
    <BottomArea />
  </div>
</template>

<script>
import colorsHandling from './components/colorsHandling.js'
import TopArea from './components/TopArea.vue'
import BottomArea from './components/BottomArea.vue'
import DashBoard from './components/DashBoard.vue'
import PWAInstallPrompt from './components/PWAInstallPrompt.vue'
import PWAUpdatePrompt from './components/PWAUpdatePrompt.vue'
import ThemeSettings from './components/ThemeSettings.vue'

export default {
  data() {
    return {
      settingsVisible: false,
    };
  },
  name: 'App',
  components: {
    TopArea,
    BottomArea,
    DashBoard,
    PWAInstallPrompt,
    PWAUpdatePrompt,
    ThemeSettings
  },
  methods: {
    showSettings(b) {
      this.settingsVisible = b;
    },
  },
  mounted() {
    this.hex = this.config.customization.colors.theme;
    this.hsl.bgs = this.config.customization.colors.items.saturation;
    this.hsl.bgl = this.config.customization.colors.items.lightness;
  },
  setup() {
    return colorsHandling;
  },
}
</script>

<style>
:root {
  --theme-color-h: 205;
  --theme-color-s: 79%;
  --theme-color-l: 13%;
  --link-contrast: .2;

  --theme-color-hsl: var(--theme-color-h), var(--theme-color-s), var(--theme-color-l);
  --theme-color: hsl(var(--theme-color-hsl));
  /* Waiting for mature compatibility. Variable calculated in JS
     https://developer.mozilla.org/en-US/docs/Web/CSS/mod#browser_compatibility
  --theme-color-contrast: hsl(var(--theme-color-h), 0%, mod(calc(calc(calc(var(--theme-color-l) - 50%) / 10) + 100%), 100%));*/ 
  --theme-color-contrast: #e0f3ff;
  --base-color-contrast: #00121f;

  --base-color-h: var(--theme-color-h);
  --base-color-s: 18%;
  --base-color-l: 80%;

  --base-color-hsl: var(--base-color-h), var(--base-color-s), var(--base-color-l);
  --base-color: hsl(var(--base-color-hsl));
  --base-color-light: hsl(var(--base-color-h), var(--base-color-s), calc(var(--base-color-l) + 10%));
  /*--base-color-light: light-dark(hsl(var(--base-color-h), var(--base-color-s), 90%), hsl(var(--base-color-h), var(--base-color-s), 20%));
  color-scheme: light dark;/*

  /* Waiting for mature compatibility. Variable calculated in JS
     https://developer.mozilla.org/en-US/docs/Web/CSS/mod#browser_compatibility
  --base-color-contrast: hsl(var(--base-color-h), var(--base-color-s), max(min(mod(calc(var(--base-color-l) + 50%), 100%), 85%), 10%));
  --base-text-color: hsl(var(--base-color-h), var(--base-color-s), max(min(mod(calc(var(--base-color-l) + 50%), 100%), 85%), 10%));*/
  /*--base-text-color: #65acdc;*/

  --theme-color-10: hsla(var(--theme-color-hsl), .1);
  --theme-color-20: hsla(var(--theme-color-hsl), .2);
  --theme-color-30: hsla(var(--theme-color-hsl), .3);
  --theme-color-40: hsla(var(--theme-color-hsl), .4);
  --theme-color-50: hsla(var(--theme-color-hsl), .5);
  --theme-color-60: hsla(var(--theme-color-hsl), .6);
  --theme-color-70: hsla(var(--theme-color-hsl), .7);
  --theme-color-80: hsla(var(--theme-color-hsl), .8);
  --theme-color-90: hsla(var(--theme-color-hsl), .9);
}

@import url('https://fonts.googleapis.com/css?family=Archivo+Black');

* {
  box-sizing: border-box;
}

body {
  background: var(--base-color);
  margin: 0;
  padding: 0;
  font-family: sans-serif;
  display: flex;
  min-height: 100vh;
  flex-direction: column;
}

textarea {
  width: 100%;
  -webkit-box-sizing: border-box;
     -moz-box-sizing: border-box;
          box-sizing: border-box;
}

a {
  color: var(--base-color-contrast);
  text-decoration-style: dotted;
  text-underline-offset: 0.2em;
  transition: color 475ms ease;
}

a:hover {
  text-decoration-style: dashed;
  color: var(--theme-color);
}

button {
  background-color: var(--theme-color);
  border: none;
  color: var(--theme-color-contrast);
  padding: 6px 10px;
  text-decoration: none;
  margin: 4px 2px;
  cursor: pointer;
}


.fade-enter-active, .fade-leave-active {
  transition: opacity .3s;
}
.fade-enter-from, .fade-leave-to {
  opacity: 0;
}
.fade-enter-to, .fade-leave-from {
  opacity: 1;
}

#container {
  margin: 0;
  padding: 0;
  display: flex;
  min-height: 100vh;
  flex-direction: column;
}

#content {
  flex: 1 0 auto;
}

 /* The switch - the box around the slider */
.switch {
  position: relative;
  display: inline-block;
  width: 40px;
  height: 24px;
}

/* Hide default HTML checkbox */
.switch input {
  opacity: 0;
  width: 0;
  height: 0;
}

/* The slider */
.slider {
  position: absolute;
  cursor: pointer;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background-color: #ccc;
  -webkit-transition: .4s;
  transition: .4s;
}

.slider:before {
  position: absolute;
  content: "";
  height: 16px;
  width: 16px;
  left: 4px;
  bottom: 4px;
  background-color: var(--theme-color-contrast);
  -webkit-transition: .4s;
  transition: .4s;
}

input:checked + .slider {
  background-color: var(--theme-color);
}

/********** Range Input Styles **********/
/*Range Reset*/
input[type="range"] {
   -webkit-appearance: none;
    appearance: none;
    background: transparent;
    cursor: pointer;
    width: 100%;
}

/* Removes default focus */
input[type="range"]:focus {
  outline: none;
}

/***** Chrome, Safari, Opera and Edge Chromium styles *****/
/* slider track */
input[type="range"]::-webkit-slider-runnable-track {
   background-color: var(--theme-color);
   border-radius: 0.5rem;
   height: 0.5rem;  
}

/* slider thumb */
input[type="range"]::-webkit-slider-thumb {
  -webkit-appearance: none; /* Override default look */
   appearance: none;
   margin-top: -12px; /* Centers thumb on the track */

   /*custom styles*/
   background-color: var(--theme-color-contrast);
   height: 1.4rem;
   width: 1rem;
}

input[type="range"]:focus::-webkit-slider-thumb {   
  border: 1px solid var(--theme-color);
  outline: 3px solid var(--theme-color);
  outline-offset: 0.125rem; 
}

/******** Firefox styles ********/
/* slider track */
input[type="range"]::-moz-range-track {
   background-color: var(--theme-color);
   border-radius: 0.5rem;
   height: 0.5rem;
}

/* slider thumb */
input[type="range"]::-moz-range-thumb {
   border: none; /*Removes extra border that FF applies*/
   border-radius: 0; /*Removes default border-radius that FF applies*/

   /*custom styles*/
   background-color: var(--theme-color-contrast);
   height: 1.4rem;
   width: 1rem;
}

input[type="range"]:focus::-moz-range-thumb {
  border: 1px solid var(--theme-color);
  outline: 3px solid var(--theme-color);
  outline-offset: 0.125rem; 
}

input:checked {
  background-color: var(--theme-color);
}

input:focus  + .slider  {
  box-shadow: 0 0 1px var(--theme-color);
}

input:checked + .slider:before {
  -webkit-transform: translateX(16px);
  -ms-transform: translateX(16px);
  transform: translateX(16px);
}
/* Rounded sliders */
.slider.round {
  border-radius: 24px;
}

.slider.round:before {
  border-radius: 50%;
}


</style>
