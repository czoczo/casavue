import { ref, watch } from 'vue'

// default values for brightness in auto mode
const autoModeLightness = ref({light: 94, dark: 6})

// Use ref to make the properties reactive
const hsl = ref({ h: 205, s: 79, l: 13, bgs: 18, bgl: 80 })
//const rgb = ref('rgb(0, 0, 0)')
const hex = ref('#07263B')

const hslToRgb = (h, s, l) => {
  s /= 100;
  l /= 100;
  let c = (1 - Math.abs(2 * l - 1)) * s,
      x = c * (1 - Math.abs((h / 60) % 2 - 1)),
      m = l - c/2,
      r = 0,
      g = 0,
      b = 0;
  if (0 <= h && h < 60) {
    r = c; g = x; b = 0;
  } else if (60 <= h && h < 120) {
    r = x; g = c; b = 0;
  } else if (120 <= h && h < 180) {
    r = 0; g = c; b = x;
  } else if (180 <= h && h < 240) {
    r = 0; g = x; b = c;
  } else if (240 <= h && h < 300) {
    r = x; g = 0; b = c;
  } else if (300 <= h && h < 360) {
    r = c; g = 0; b = x;
  }
  r = Math.round((r + m) * 255);
  g = Math.round((g + m) * 255);
  b = Math.round((b + m) * 255);
  return { r, g, b };
}

const rgbToHsl = (r, g, b) => {
  r /= 255;
  g /= 255;
  b /= 255;
  let max = Math.max(r, g, b), min = Math.min(r, g, b);
  let h, s, l = (max + min) / 2;
  if(max == min){
    h = s = 0;
  } else {
    let d = max - min;
    s = l > 0.5 ? d / (2 - max - min) : d / (max + min);
    switch(max){
      case r: h = (g - b) / d + (g < b ? 6 : 0); break;
      case g: h = (b - r) / d + 2; break;
      case b: h = (r - g) / d + 4; break;
    }
    h /= 6;
  }
  const bgs = hsl.value.bgs;
  const bgl = hsl.value.bgl;
  return { h: h*360, s: s*100, l: l*100, bgs: bgs, bgl: bgl };
}

const rgbToHex = (r, g, b) => {
  return '#' + [r, g, b].map(x => {
    const hex = x.toString(16)
    return hex.length === 1 ? '0' + hex : hex
  }).join('')
}

const hexToRgb = (hex) => {
  let r = parseInt(hex.slice(1, 3), 16),
      g = parseInt(hex.slice(3, 5), 16),
      b = parseInt(hex.slice(5, 7), 16);
  return { r, g, b }
}

const getThemeLightness = () => {
    if (window.matchMedia('(prefers-color-scheme: dark)').matches) {
      // dark mode
      return autoModeLightness.value.dark
    } else if (window.matchMedia('(prefers-color-scheme: light)').matches) {
      // light mode
      return autoModeLightness.value.light
    } else {
      return 50;
    }
}

const updateRGB = () => {
  var c = hslToRgb(hsl.value.h, hsl.value.s, hsl.value.l);
  hex.value = rgbToHex(c['r'],c['g'],c['b']);

  let bgl = hsl.value.bgl;
  // -1 == auto lightness mode
  if (bgl == -1 && window.matchMedia) {
    bgl = getThemeLightness();
  }

  document.documentElement.style.setProperty('--theme-color-h', hsl.value.h);
  document.documentElement.style.setProperty('--theme-color-s', hsl.value.s + "%");
  document.documentElement.style.setProperty('--theme-color-l', hsl.value.l + "%");
  document.documentElement.style.setProperty('--base-color-s', hsl.value.bgs + "%");
  document.documentElement.style.setProperty('--base-color-l', bgl + "%");
  c = hslToRgb(hsl.value.h, hsl.value.s, hsl.value.l > 50 ? autoModeLightness.value.dark : autoModeLightness.value.light);
  document.documentElement.style.setProperty('--theme-color-contrast', rgbToHex(c['r'],c['g'],c['b']));
  c = hslToRgb(hsl.value.h, hsl.value.s * 0.8, Math.max( Math.min( ((hsl.value.l + 50) % 100), 85 ), 10));
  document.documentElement.style.setProperty('--base-text-color', rgbToHex(c['r'],c['g'],c['b']));
  c = hslToRgb(hsl.value.h, hsl.value.bgs, bgl > 50 ? autoModeLightness.value.dark : autoModeLightness.value.light);
  document.documentElement.style.setProperty('--base-color-contrast', rgbToHex(c['r'],c['g'],c['b']));
}

const updateHSL = () => {
  if ( ! hex.value.startsWith('#')) {
    hex.value = "#" + hex.value;
  }
  if (hex.value.length > 7) {
    hex.value = hex.value.substring(0, 7);
  }
  const regex = /^#[A-Fa-f0-9]{6}$/;
  if ( ! regex.test(hex.value) ) {
    return
  }
  const { r, g, b } = hexToRgb(hex.value)
  hsl.value = rgbToHsl(r, g, b)
  document.documentElement.style.setProperty('--theme-color-h', hsl.value.h);
  document.documentElement.style.setProperty('--theme-color-s', hsl.value.s + "%");
  document.documentElement.style.setProperty('--theme-color-l', hsl.value.l + "%");
}

watch(hsl, updateRGB)
watch(hex, updateHSL)

export default { hsl, hex, updateRGB, updateHSL, getThemeLightness }
