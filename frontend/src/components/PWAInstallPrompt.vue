<!-- PWAInstallPrompt.vue -->
<template>
  <div class="pwa-prompt" v-if="shown">
    Add app to home screen?

    <button @click="installPWA">
      Install!
    </button>

    <button @click="dismissPrompt">
      No, thanks
    </button>
  </div>
</template>

<script>
export default {
  data: () => ({
    shown: false,
  }),

  beforeMount() {
    window.addEventListener('beforeinstallprompt', (e) => {
      e.preventDefault()
      this.installEvent = e
      this.shown = true
    })
  },

  methods: {
    dismissPrompt() {
      this.shown = false
    },

    installPWA() {
      this.installEvent.prompt()
      this.installEvent.userChoice.then((choice) => {
        this.dismissPrompt() // Hide the prompt once the user's clicked
        if (choice.outcome === 'accepted') {
          // Do something additional if the user chose to install
        } else {
          // Do something additional if the user declined
        }
      })
    },
  }
}
</script>

<style>
.pwa-prompt {
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
