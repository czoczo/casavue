<template>
    <div id="settings">
      <div class="iconsTopRight">
        <button id="settingsClose" @click="$emit('show-settings', false)">
          <font-awesome-icon icon="fa-solid fa-xmark" />
        </button>
      </div>
      <h2>Theme color:</h2>
      <div class="form-items">
        <label>RGB:</label>
        <input v-model="hex" @input="updateHSL" />

        <label>Hue:</label>
        <input type="range" min="0" max="359" v-model="hsl.h" @input="updateRGB" />

        <label>Saturation:</label>
        <input type="range" min="0" max="100" v-model="hsl.s" @input="updateRGB" />

        <label>Lightness:</label>
        <input type="range" min="0" max="100" v-model="hsl.l" @input="updateRGB" />
      </div>

      <h2>Items look:</h2>
      <div class="form-items">
        <label>Saturation:</label>
        <input type="range" min="0" max="100" v-model="hsl.bgs" @input="updateRGB" />

        <label>Lightness:</label>
        <div>
          AUTO mode
          <span v-bind:class="lightnessAutoChecked ? 'modeOn' : 'modeOff'">{{ lightnessAutoChecked ? "🟢" : "🔴"  }}</span>
          <span id="autoCheckbox">
            <input type="checkbox" id="checkbox" v-model="lightnessAutoChecked" @change="setLightnessAuto" />
          </span>
        </div>

        <label></label>
        <input type="range" min="0" max="100" v-model="hsl.bgl" @input="updateRGB" :style="{visibility: lightnessAutoChecked ? 'hidden' : 'visible'}"/>
        
      </div>
      <hr>
      <div class="input-wrap">
        <h3>Customization config:</h3>
	<p>Use to persist customization changes.<br/>Copy and paste to CasaVue config.</p>
	<textarea id="w3review" name="w3review" rows="5" cols="50" v-model="settingsConfig" readonly ></textarea>
      </div>
    </div>
</template>



<script>
import colorsHandling from './colorsHandling.js'

export default {
  setup() {
    return colorsHandling;
  },
  data() {
    return {
      lightnessAutoChecked: this.hsl.bgl == -1,
    };
  },
  computed: {
    settingsConfig() {	
      return `  colors:
    theme: "${this.hex}"
    items:
      saturation: ${this.hsl.bgs}
      lightness: ${this.hsl.bgl}`;
    },
  },
  methods: {
    setLightnessAuto() {
      if (this.lightnessAutoChecked) {
        this.hsl.bgl = -1;
      } else {
        this.hsl.bgl = this.getThemeLightness();
      }
      this.updateRGB();
    },
  },
}
</script>

<style>
.form-items {
  display: grid;
  row-gap: 0.6rem;
  grid-template-columns: auto auto;
}


#settings {
  z-index: 3;
  color: var(--base-color-contrast);
  background-color: var(--base-color-light);
  border: 1px var(--theme-color) solid;
  text-align: left;
  position: fixed;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  padding: 16px;
  box-shadow: 0 4px 8px 0 rgba(0, 0, 0, 0.2), 0 6px 20px 0 rgba(0, 0, 0, 0.19);
}

#searchInput {
  background: transparent url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='16' height='16' class='bi bi-search' viewBox='0 0 16 16'%3E%3Cpath d='M11.742 10.344a6.5 6.5 0 1 0-1.397 1.398h-.001c.03.04.062.078.098.115l3.85 3.85a1 1 0 0 0 1.415-1.414l-3.85-3.85a1.007 1.007 0 0 0-.115-.1zM12 6.5a5.5 5.5 0 1 1-11 0 5.5 5.5 0 0 1 11 0z'%3E%3C/path%3E%3C/svg%3E") no-repeat 13px center;
  background-color: white;
  padding-left: 40px;
  border: none;
  padding: 6px 4px 6px 40px;
}

#w3review {
  background-color: #eee;
  border: 2px solid #ccc;
  border-radius: 4px;
  resize: none;
  padding: 0.5rem;
}
#settingsClose {
  position: absolute;
  right: 15px;
}

input:focus {
    outline: none;
}
      
#autoCheckbox {
  float: right;
}

@media (max-width: 360px), (max-height: 588px) {
  #settings {
    position: relative;
    top: 0;
    left: 0;
    transform: none;
  }
}

@media (max-width: 240px) {
  .form-items {
    grid-template-columns: none;
  }
}
</style>
