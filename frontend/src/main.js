import { createApp } from 'vue'
import App from './App.vue'
import './registerServiceWorker'

/* import the fontawesome core */
import { library } from '@fortawesome/fontawesome-svg-core'

/* import font awesome icon component */
import { FontAwesomeIcon } from '@fortawesome/vue-fontawesome'

/* import specific icons */
import { faBrush } from '@fortawesome/free-solid-svg-icons'
import { faGear } from '@fortawesome/free-solid-svg-icons'
import { faCircleXmark } from '@fortawesome/free-solid-svg-icons'
import { faXmark } from '@fortawesome/free-solid-svg-icons'
import { faThumbsUp } from '@fortawesome/free-solid-svg-icons'
import { faLock } from '@fortawesome/free-solid-svg-icons'
import { faTriangleExclamation } from '@fortawesome/free-solid-svg-icons'
import { faLockOpen } from '@fortawesome/free-solid-svg-icons'
import { faCircleQuestion } from '@fortawesome/free-solid-svg-icons'
import { faMinimize } from '@fortawesome/free-solid-svg-icons'
import { faMaximize } from '@fortawesome/free-solid-svg-icons'

/* add icons to the library */
library.add(faBrush)
library.add(faGear)
library.add(faCircleXmark)
library.add(faXmark)
library.add(faThumbsUp)
library.add(faLock)
library.add(faTriangleExclamation)
library.add(faLockOpen)
library.add(faCircleQuestion)
library.add(faMinimize)
library.add(faMaximize)

//createApp(App)
//.component('font-awesome-icon', FontAwesomeIcon)
//.mount('#app')


// VUE 3 Version
const app = createApp(App)

// ?t=Date prevents cache
//fetch(process.env.BASE_URL + "config.json?t=" + Date.now() )
fetch(process.env.BASE_URL + "config.json")
  .then((response) => response.json())
  .then((config) => {
    // either use window.config
    window.config = config
    // or use [Vue Global Config][1]
    app.config.globalProperties.config = config
    // FINALLY, mount the app
    app.component('font-awesome-icon', FontAwesomeIcon)
    app.mount("#app")
  })
