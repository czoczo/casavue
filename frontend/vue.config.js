const { defineConfig } = require('@vue/cli-service')
module.exports = defineConfig({
  transpileDependencies: true,
  //publicPath: process.env.NODE_ENV === 'production'
  //  ? '/demo/'
  //  : '/',
  publicPath: './',
  pwa: {
    //workboxOptions: {
    //  skipWaiting: true
    //},
    name: "CasaVue",
    themeColor: "#fe0102",
    msTileColor: "#fe0102",
    manifestOptions: {
      background_color: "#fe0102"
    }
  }
})
