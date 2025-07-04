<template>
  <div class="embed-container">
    <h3 v-if="title">{{ title }}</h3>
    <div class="iframe-wrapper">
      <iframe
          :src="url"
          :title="iframeTitle"
          frameborder="0"
          scrolling="auto"
          class="embedded-iframe"
          @load="onLoad"
          @error="onError"
          :style="iframeStyle"
      ></iframe>
    </div>
    <div v-if="loading" class="loading-indicator">
      <div class="spinner"></div>
      <p>加载中...</p>
    </div>
    <p v-if="error" class="error-message">{{ error }}</p>
  </div>
</template>

<script setup>
import { ref, onMounted, watch } from 'vue'

const props = defineProps({
  url: {
    type: String,
    required: true
  },
  title: {
    type: String,
    default: ''
  },
  iframeTitle: {
    type: String,
    default: '外部内容'
  },
  width: {
    type: String,
    default: '100%'
  },
  height: {
    type: String,
    default: '100%'
  }
})

const loading = ref(true)
const error = ref(null)

const onLoad = () => {
  loading.value = false
  error.value = null
}

const onError = (event) => {
  loading.value = false
  error.value = `加载失败: ${event.message || '无法加载该 URL'}`
}

// 监听 URL 变化，重新加载
watch(() => props.url, () => {
  loading.value = true
  error.value = null
})

// 设置 iframe 样式
const iframeStyle = {
  width: props.width,
  height: props.height
}
</script>

<style scoped>
.embed-container {
  margin: 1rem 0;
  border: 0 solid #e5e7eb;
  border-radius: 0.5rem;
  overflow: hidden;
}

.iframe-wrapper {
  position: relative;
  width: 100%;
}

.embedded-iframe {
  display: block;
  width: 100%;
  min-height: 400px;
  transition: opacity 0.3s ease;
}

.loading-indicator {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background-color: rgba(255, 255, 255, 0.8);
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  z-index: 10;
}

.spinner {
  width: 2rem;
  height: 2rem;
  border: 3px solid #f3f4f6;
  border-radius: 50%;
  border-top-color: #3b82f6;
  animation: spin 1s linear infinite;
  margin-bottom: 0.5rem;
}

@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}

.error-message {
  color: #ef4444;
  padding: 1rem;
  margin: 0;
  background-color: #fee2e2;
  text-align: center;
}
</style>