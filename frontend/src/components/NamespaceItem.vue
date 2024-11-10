<template>
  <a class="item" target="_blank" :href="item.data.url">
    <div class="item-content">
      <div class="image-container">
        <img :src="item.data.iconURL" alt="🖻" />
      </div>
      <div class="item-text">
        <div class="item-name">{{ getItemName(item) }}</div>
        <div class="item-description">{{ item.data.description }}</div>
      </div>
      <div class="circle-status">
        <div class="status-icon" v-if="! isTLS(item.data.url)">
          <font-awesome-icon icon="fa-solid fa-lock-open" />
          <span class="tooltiptext">No TLS encryption</span>
        </div>
        <div class="status-icon" v-if="isSiteUnavailable(item.name)">
          <font-awesome-icon icon="fa-solid fa-exclamation-triangle" />
          <span class="tooltiptext">Site unavailable</span>
        </div>
        <div class="status-icon" v-if="isStatusUnavailable(item.name)">
          <font-awesome-icon icon="fa-solid fa-circle-question" />
          <span class="tooltiptext">Status unknow</span>
        </div>
      </div>
    </div>
  </a>
</template>

<script>
export default {
  data() {
    return {
      staticMode: false
    };
  },
  mounted() {
    this.staticMode = this.config.staticMode;
  },
  props: {
    item: Object,
    itemsStatus: Object,
  },
  methods: {
    isTLS(url) {
      return url.startsWith('https');
    },
    isStatusUnavailable(status) {
      return (!this.config.staticMode) && this.itemsStatus[status].status == 'gray';
    },
    isSiteUnavailable(status) {
      return this.itemsStatus[status].status == 'red';
    },
    getItemName(item) {
      if (!("labels" in item.data)) {
        return item.name
      } 
        
      if (!("app.kubernetes.io/name" in item.data.labels)) {
        return item.name
      }
      
      return item.data.labels["app.kubernetes.io/name"]
    },
  },
}
</script>

<style>
.item {
  margin: 4px;
  padding: 6px;
  background-color: var(--base-color-light);
  -webkit-transition: background 1s; 
  transition: background 1s;
  box-shadow: rgba(60, 64, 67, 0.3) 0px 1px 2px 0px, rgba(60, 64, 67, 0.15) 0px 2px 6px 2px;
}

.item-content {
  display: flex;
  position: relative;
  justify-content: space-between;
  color: var(--theme-color-lowest);
  height: 100%;
}

.item:hover {
  background-color: var(--theme-color-50);
}
/*.item.item-content.circle-status svg{*/
.item:hover svg{
  stroke: var(--theme-color-50);
}

.image-container {
  margin-right: 14px;
  flex: 0 0 auto; /* Make sure the image container doesn't grow */
  display: flex;
  align-items: top;
}

.image-container img {
  height: 50px;
  font-size: 40px;
  line-height: 60px;
  color: var(--base-color-contrast);
}

.item-text {
  color: var(--base-color-contrast);
  flex: 1; /* Allow the title container to grow and take remaining space */
  display: flex;
  flex-direction: column;
  align-items: left;
  justify-content: center;
  margin-top: 8px;
  margin-bottom: 8px;
}

.item-name {
  font-weight: bold;
}

.item-description {
}

.item:nth-child(2n-1):last-child {
  grid-column: 1 / -1;
}

.circle-status {
  font-size: 0.8em;
  position: absolute;
  right: -2px;
  bottom: -4px;
  color: var(--theme-color);
}

.circle-status svg {
  margin: 0px 0px 0px 2px;
  stroke: var(--theme-color-contrast);
  stroke-width: 25;
  -webkit-transition: stroke 1s; 
  transition: stroke 1s;
}

.circle-status div {
  display: inline-block;
}

.status-icon {
  position: relative;
}
.status-icon .tooltiptext {
  opacity: 0;
  width: 120px;
  background-color: var(--theme-color);
  color: var(--theme-color-contrast);
  text-align: center;
  padding: 5px 0;
  border-radius: 6px;
  position: absolute;
  z-index: 100;
  bottom: 150%;
  left: 100%;
  margin-left: -110px;
  -webkit-transition: opacity 0.4s; 
  transition: opacity 0.4s;
}

.status-icon .tooltiptext::after {
  content: "";
  position: absolute;
  top: 100%;
  left: 84%;
  margin-left: -3px;
  border-width: 5px;
  border-style: solid;
  border-color: var(--theme-color) transparent transparent transparent;
}

/* Show the tooltip text when you mouse over the tooltip container */
.status-icon:hover .tooltiptext {
  opacity: 1;
}
</style>
