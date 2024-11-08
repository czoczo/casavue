<template>
  <div id="toolbox">
  <input id="searchInput" v-model="searchText" placeholder="Search by name" autofocus /> | 
  <button @click="showAllNamespaces">
    <font-awesome-icon icon="fa-solid fa-maximize" />
  </button>
  <button @click="hideAllNamespaces">
    <font-awesome-icon icon="fa-solid fa-minimize" />
  </button>
  </div>

  <ErrorBox v-if="dataMissing"/>

  <div id="items">

    <div v-for="namespace in sortedNamespaces" :key="namespace" class="namespace-container">
      <div class="namespace-header">
        <div class="namespace-name">{{ namespace }}</div>
        <div class="input-wrap">
          <label class="switch">
            <input type="checkbox" v-model="visibleNamespaces[namespace]">
            <span class="slider round"></span>
          </label>
        </div>
      </div>

      <div class="namespace-items" v-bind:class="{'hidden': !isNamespaceVisible(namespace)}">
        <NamespaceItem v-for="item in groupedData[namespace]" :key="item.name" :item="item" :itemsStatus="itemsStatus" />
      </div>

    </div>
  </div>

</template>

<script>
import axios from 'axios';
import ErrorBox from './ErrorBox.vue'
import NamespaceItem from './NamespaceItem.vue'

export default {
  data() {
    return {
      /*apiBaseUrl: process.env.NODE_ENV === 'development' ? 'http://localhost:3001/apiv1' : './api/v1',*/
      apiBaseUrl: process.env.NODE_ENV === 'development' ? './api/v1' : './api/v1',
      data: {},
      searchText: '',
      visibleNamespaces: {},
      itemsStatus: {},
      refreshCounter: 0,
    };
  },

  components: {
    NamespaceItem,
    ErrorBox,
  },
  computed: {
    filteredData() {
      if (!this.searchText) {
        return this.data;
      }

      const searchLowerCase = this.searchText.toLowerCase();
      const filteredRecords = {};

      for (const key in this.data) {
        if (key.toLowerCase().includes(searchLowerCase)) {
          filteredRecords[key] = this.data[key];
        }
      }
      return filteredRecords;
    },

    groupedData() {
      const grouped = {};
      for (var key in this.filteredData){
        if (!grouped[this.filteredData[key].namespace]) {
          grouped[this.filteredData[key].namespace] = [];
        }
        grouped[this.filteredData[key].namespace].push({name: key, data: this.filteredData[key]});
      }


      return grouped;
    },

    sortedNamespaces() {
      const uniqueNamespaces = new Set();

        // Iterate through the object and collect unique "namespace" values
        for (const key in this.data) {
          if ((Object.prototype.hasOwnProperty.call(this.data, key)) && (this.groupedData[this.data[key].namespace])) {
            uniqueNamespaces.add(this.data[key].namespace);
          }
        }

        // Convert the set to an array and sort alphabetically
        const sortedNamespaces = Array.from(uniqueNamespaces).sort();

        return sortedNamespaces;
    },
    
    dataMissing() {
      return !Object.keys(this.data).length
    }
  },
  methods: {
    fetchData() {
      axios.get(`${this.apiBaseUrl}`)
        .then(response => {
          this.data = response.data;
          for (var key in this.data){
            if (this.visibleNamespaces[this.data[key].namespace] == null) {
              this.visibleNamespaces[this.data[key].namespace] = true;
            }
            if (!this.itemsStatus[key]) {
              this.itemsStatus[key] = {name: key, url: this.data[key].url, status: "gray"};
            }
          }

          // refresh ingress status every 3 minutes (180sec / 5sec of API refresh time)
          if ((!this.config.staticMode) && (this.refreshCounter % 36 == 0)) {
            this.checkSiteAvailability();
          }
          this.refreshCounter++;

        })
        .catch(error => {
          console.error('Error fetching data:', error);
        });

    },
    hideAllNamespaces() {
      Object.keys(this.visibleNamespaces).forEach((namespace) => {
        this.visibleNamespaces[namespace] = false
      });
    },
    showAllNamespaces() {
      Object.keys(this.visibleNamespaces).forEach((namespace) => {
        this.visibleNamespaces[namespace] = true
      });
    },
    isNamespaceVisible(namespace) {
      return this.visibleNamespaces[namespace];
    },



    async checkSiteAvailability() {
      for (const item in this.itemsStatus) {
        try {
          const response = await axios.head('/statusCheck/?url=' + this.itemsStatus[item].url);
          this.updateSiteStatus(item, response.status);
        } catch (error) {
          this.updateSiteStatus(item, error.response ? error.response.status : 'unknown');
        }
      }
    },
    updateSiteStatus(name, status) {
      const validStatusCodes = [200, 401];
      const siteStatus = validStatusCodes.includes(status) ? 'green' : 'red';
      this.itemsStatus[name] = {name: name, url: this.data[name].url, status: siteStatus};
    },
  },
  created() {
    this.fetchData();
    // Fetch data every 5 seconds
    setInterval(this.fetchData, 5000);
  },
};
</script>

<style scoped>
a:link { text-decoration: none; }
a:visited { text-decoration: none; }
a:hover { text-decoration: none; }
a:active { text-decoration: none; }

#toolbox {
  background: var(--theme-color);
  padding: 4px 0px;
  width: 100%;
  display: block;
  text-align: right;
  position: -webkit-sticky; /* Safari */
  position: sticky;
  top: 0;
  z-index: 1;
  box-shadow: 0px 4px 3px 1px rgb(0 0 0 / 20%);
}

#items {
  font-family: Avenir, Helvetica, Arial, sans-serif;
  -webkit-font-smoothing: antialiased;
  -moz-osx-font-smoothing: grayscale;
  gap: .1rem;
  margin: 0 auto;
  overflow: auto;
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(480px, 1fr));
}

.namespace-container {
  margin: 4px;
}

.namespace-items {
  transform: scaleY(1);
  display: grid;
  grid-template-columns: repeat(var(--col-count,2),minmax(0,1fr));
}

.hidden {
  max-height: 0;
  transform: scaleY(0);
}

.namespace-header {
  padding: 5px;
  font-weight: bold;
  font-size: 1.5rem;
  display: grid;
  gap: .1rem;
  margin: 0 auto;
  overflow: auto;
  display: grid;
  grid-template-columns: 3fr 1fr;
}

.namespace-name {
  display: inline-block;
  color: var(--base-color-contrast);
}

.input-wrap {
  height: 0px;
  text-align: right;
}
@media (max-width: 480px) {
  #app {
    grid-template-columns: 1fr;
  }

  #items {
    grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));
  }

  .namespace-items {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 240px) {
  #items {
    grid-template-columns: repeat(auto-fill, minmax(120px, 1fr));
  }
}
</style>

