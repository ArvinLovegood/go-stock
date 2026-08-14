<script setup>
import { MdPreview } from 'md-editor-v3';
import 'md-editor-v3/lib/preview.css';
import {h, computed, nextTick, onBeforeUnmount, onMounted, ref} from 'vue';
import {CheckUpdate, GetConfig, GetVersionInfo,GetSponsorInfo,GetUserManual,OpenURL,RestartAsAdmin} from "../../wailsjs/go/main/App";
import {EventsOff, EventsOn,Environment} from "../../wailsjs/runtime";
import {NAvatar, NButton, NTree, useNotification,NText} from "naive-ui";
import { addMonths, format ,parse} from 'date-fns';
import { zhCN } from 'date-fns/locale';
const updateLog = ref('');
const versionInfo = ref('');
const icon = ref('https://raw.githubusercontent.com/ArvinLovegood/go-stock/master/build/appicon.png');
const alipay =ref('https://github.com/ArvinLovegood/go-stock/raw/master/build/screenshot/alipay.jpg')
const wxpay =ref('https://github.com/ArvinLovegood/go-stock/raw/master/build/screenshot/wxpay.jpg')
const wxgzh =ref('https://github.com/ArvinLovegood/go-stock/raw/dev/build/screenshot/%E6%89%AB%E7%A0%81_%E6%90%9C%E7%B4%A2%E8%81%94%E5%90%88%E4%BC%A0%E6%92%AD%E6%A0%B7%E5%BC%8F-%E7%99%BD%E8%89%B2%E7%89%88.png')
const notify = useNotification()
const vipLevel=ref("");
const vipStartTime=ref("");
const vipEndTime=ref("");
const expired=ref(false)
const showManual = ref(false)
const manualContent = ref('')
const manualId = 'manual-preview'
const darkTheme = ref(false)
const theme = computed(() => darkTheme.value ? 'dark' : 'light')
const manualScrollRef = ref(null)
const catalogList = ref([])

const buildCatalogTree = (headings) => {
  if (!headings.length) return []
  const roots = []
  const stack = []
  for (const h of headings) {
    const node = { key: h.text, label: h.text, level: h.level, children: [] }
    while (stack.length && stack[stack.length - 1].level >= h.level) {
      stack.pop()
    }
    if (stack.length) {
      stack[stack.length - 1].children.push(node)
    } else {
      roots.push(node)
    }
    stack.push(node)
  }
  const clean = (nodes) => {
    for (const n of nodes) {
      if (n.children.length === 0) delete n.children
      else clean(n.children)
    }
  }
  clean(roots)
  return roots
}

const catalogTree = computed(() => buildCatalogTree(catalogList.value))

const onTreeSelect = (keys) => {
  if (keys.length) scrollToHeading(keys[0])
}

const slugifyHeading = (text) => {
  return text
    .trim()
    .replace(/[^\w\u4e00-\u9fff]/g, '-')
    .replace(/-+/g, '-')
    .replace(/^-|-$/g, '')
}

const extractCatalog = () => {
  if (!manualScrollRef.value) return
  const headings = manualScrollRef.value.querySelectorAll('h1, h2, h3, h4, h5, h6')
  catalogList.value = Array.from(headings).map(h => ({
    text: h.textContent?.trim() || '',
    level: parseInt(h.tagName.slice(1))
  }))
}

const scrollToHeading = (headingText) => {
  if (!manualScrollRef.value) return
  const container = manualScrollRef.value
  const headings = container.querySelectorAll('h1, h2, h3, h4, h5, h6')
  for (const h of headings) {
    const text = h.textContent?.trim()
    if (text === headingText) {
      const containerRect = container.getBoundingClientRect()
      const headingRect = h.getBoundingClientRect()
      container.scrollTop += headingRect.top - containerRect.top - 10
      return
    }
  }
}

const openManual = () => {
  if (!manualContent.value) {
    GetUserManual().then(res => {
      manualContent.value = res
      showManual.value = true
      nextTick(() => { setTimeout(extractCatalog, 500) })
    })
  } else {
    showManual.value = true
    nextTick(() => { setTimeout(extractCatalog, 300) })
  }
}

onMounted(() => {
  document.title = '关于软件';
  GetConfig().then(res => {
    darkTheme.value = res.darkTheme
  })
  GetVersionInfo().then((res) => {
    updateLog.value = res.content;
    versionInfo.value = res.version;
    icon.value = res.icon;
    alipay.value=res.alipay;
    wxpay.value=res.wxpay;
    wxgzh.value=res.wxgzh;

    GetSponsorInfo().then((res) => {
      vipLevel.value = res.vipLevel;
      vipStartTime.value = res.vipStartTime;
      vipEndTime.value = res.vipEndTime;
      //判断时间是否到期
      if (res.vipLevel) {
        if (res.vipEndTime < format(new Date(), 'yyyy-MM-dd HH:mm:ss')) {
          notify.warning({content: 'VIP已到期'})
          expired.value = true;
        }
      }
    })

  });



})
onBeforeUnmount(() => {
  notify.destroyAll()
  EventsOff("updateVersion")
  EventsOff("updateNeedAdmin")
})

EventsOn("updateVersion",async (msg) => {
  const githubTimeStr = msg.published_at;
  const utcDate = new Date(githubTimeStr);
  const date = new Date(utcDate.getTime());
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, '0');
  const day = String(date.getDate()).padStart(2, '0');
  const hours = String(date.getHours()).padStart(2, '0');
  const minutes = String(date.getMinutes()).padStart(2, '0');
  const seconds = String(date.getSeconds()).padStart(2, '0');

  const formattedDate = `${year}-${month}-${day} ${hours}:${minutes}:${seconds}`;

  notify.info({
    avatar: () =>
        h(NAvatar, {
          size: 'small',
          round: false,
          src: icon.value
        }),
    title: '发现新版本: ' + msg.tag_name,
    content: () => {
      return h('div', {
        style: {
          'text-align': 'left',
          'font-size': '14px',
        }
      }, { default: () => msg.commit?.message })
    },
    duration: 5000,
    meta: "发布时间:"+formattedDate,
    action: () => {
      return h(NButton, {
        type: 'primary',
        size: 'small',
        onClick: () => {
          Environment().then(env => {
            switch (env.platform) {
              case 'windows':
                window.open(msg.html_url)
                break
              default :
                OpenURL(msg.html_url)
                break
            }
          })
        }
      }, { default: () => '查看' })
    }
  })
})

EventsOn("updateNeedAdmin", (msg) => {
  notify.warning({
    avatar: () =>
        h(NAvatar, {
          size: 'small',
          round: false,
          src: icon.value
        }),
    title: '更新需要管理员权限',
    content: () => {
      return h('div', {
        style: {
          'text-align': 'left',
          'font-size': '14px',
        }
      }, { default: () => '新版本 ' + (msg.version || '') + ' 下载完成，但自动替换文件需要管理员权限。请以管理员身份重启程序后再次检查更新。' })
    },
    duration: 15000,
    action: () => {
      return h(NButton, {
        type: 'warning',
        size: 'small',
        onClick: () => {
          RestartAsAdmin()
        }
      }, { default: () => '以管理员身份重启' })
    }
  })
})

</script>

<template>
      <n-space vertical size="large"  style="--wails-draggable:no-drag">
        <!-- 软件描述 -->
        <n-card size="large">
          <n-divider title-placement="center">关于软件</n-divider>
          <n-space vertical >
            <n-image width="100" :src="icon" />
            <h1>
              <n-badge v-if="!vipLevel"  :value="versionInfo" :offset="[80,10]"  type="success">
                <n-gradient-text type="info" :size="50" >go-stock</n-gradient-text>
              </n-badge>
              <n-badge v-if="vipLevel"  :value="versionInfo" :offset="[70,10]"  type="success">
                <n-gradient-text :type="expired?'error':'warning'" :size="50" >go-stock</n-gradient-text><n-tag :bordered="false" size="small" type="warning">VIP{{vipLevel}}</n-tag>
              </n-badge>
            </h1>
            <n-gradient-text  :type="expired?'error':'warning'" v-if="vipLevel" >vip到期时间：{{vipEndTime}}</n-gradient-text>
            <n-flex justify="center">
              <n-button size="tiny" @click="CheckUpdate(1)"  type="info" tertiary >检查更新</n-button>
              <n-button size="tiny" @click="openManual" type="success" tertiary >查看用户手册</n-button>
            </n-flex>
            <div style="justify-self: center;text-align: left" >
              <p>自选股行情实时监控，基于Wails和NaiveUI构建的AI赋能股票分析工具</p>
              <p>目前已支持A股，港股，美股，未来计划加入基金，ETF等支持</p>
              <p>支持DeepSeek，OpenAI， Ollama，LMStudio，AnythingLLM，<a href="https://cloud.siliconflow.cn/i/foufCerk" target="_blank">硅基流动</a>，<a href="https://www.volcengine.com/experience/ark?utm_term=202502dsinvite&ac=DSASUQY5&rc=IJSE43PZ" target="_blank">火山方舟</a>，阿里云百炼等平台或模型</p>
              <p>
                <i style="color: crimson">本软件仅供学习研究目的，AI分析结果仅供参考，本软件不提供任何投资建议或决策，风险自担！</i>
              </p>
              <p>
                欢迎点赞GitHub：<a href="https://github.com/ArvinLovegood/go-stock" target="_blank">go-stock</a><n-divider vertical />
                <a href="https://github.com/ArvinLovegood/go-stock" target="_blank">GitHub</a><n-divider vertical />
                <a href="https://github.com/ArvinLovegood/go-stock/issues" target="_blank">Issues</a><n-divider vertical />
                <a href="https://github.com/ArvinLovegood/go-stock/releases" target="_blank">Releases</a><n-divider vertical />
              </p>
              <p v-if="updateLog">更新说明：{{updateLog}}</p>
              <p>项目社区：<a href="https://go-stock.sparkmemory.top/" target="_blank">https://go-stock.sparkmemory.top/</a></p>
              <p>QQ交流群：<a href="http://qm.qq.com/cgi-bin/qm/qr?_wv=1027&k=0YQ8qD3exahsD4YLNhzQTWe5ssstWC89&authKey=usOMMRFtIQDC%2FYcatHYapcxQbJ7PwXPHK9OypTXWzNjAq%2FRVvQu9bj2lRgb%2BSZ3p&noverify=0&group_code=491605333" target="_blank">491605333</a></p>
            </div>
          </n-space>
          <n-divider title-placement="center">支持💕开源</n-divider>
          <n-flex justify="center">
            <n-table  size="small" style="width: 820px">
              <n-thead>
                <n-tr>
                  <n-th>赞助计划</n-th>
                  <n-th>赞助等级</n-th>
                  <n-th>权益说明</n-th>
                </n-tr>
              </n-thead>
              <n-tbody>
                <n-tr>
                  <n-td>每月 0 RMB</n-td><n-td>vip0</n-td><n-td>🌟 全部功能,软件自动更新(从GitHub下载),自行解决github平台网络问题。</n-td>
                </n-tr>
                <n-tr>
                  <n-td><s>赞助 18.8 RMB/月<br>赞助 120 RMB/年</s></n-td><n-td><s>vip1</s></n-td><n-td><s>💕 全部功能,软件自动更新(从CDN下载),更新快速便捷。AI配置指导，提示词参考等</s> <n-tag size="tiny" type="error">已停售，推荐VIP2</n-tag></n-td>
                </n-tr>
                <n-tr>
                  <n-td>赞助 28.8 RMB/月<br>赞助 240 RMB/年</n-td><n-td>vip2</n-td><n-td>💕 vip0全部功能,启动时自动同步最近24小时市场资讯(包括外媒简讯)，go-stock Ai助手等(详询作者微信/QQ)💕</n-td>
                </n-tr>
                <n-tr>
                  <n-td>每月赞助 X RMB</n-td><n-td>vipX</n-td><n-td>🧩 更多计划，视go-stock开源项目发展情况而定...(承接GitHub项目README广告推广💖)</n-td>
                </n-tr>
              </n-tbody>
            </n-table>
          </n-flex>
          <n-divider title-placement="center">关于作者</n-divider>
          <n-space vertical>
<!--            <h1>关于作者</h1>-->
            <n-avatar width="100" src="https://avatars.githubusercontent.com/u/7401917?v=4" />
            <h2><a href="https://github.com/ArvinLovegood" target="_blank">@ArvinLovegood</a></h2>
            <p>一个热爱编程的小白，欢迎关注我的Github/微信公众号</p>
            <n-image width="300" :src="wxgzh" />
            <p>开源不易，如果觉得好用，可以请作者喝杯咖啡。</p>
            <n-flex justify="center">
              <n-image width="200" :src="alipay" />
              <n-image width="200" :src="wxpay" />
            </n-flex>
          </n-space>
          <n-divider title-placement="center">鸣谢</n-divider>
          <div style="justify-self: center;text-align: left" >
            <p>
              感谢以下捐赠者：
              <n-gradient-text size="small" type="warning">*晨</n-gradient-text><n-divider vertical />
            </p>
            <p>
              感谢以下开发者：
              <a href="https://github.com/GiCo001" target="_blank">@Gico</a><n-divider vertical />
              <a href="https://github.com/CodeNoobLH" target="_blank">浓睡不消残酒</a><n-divider vertical />
              <a href="https://github.com/gnim2600" target="_blank">@gnim2600</a><n-divider vertical />
              <a href="https://github.com/XXXiaohuayanGGG" target="_blank">@XXXiaohuayanGGG</a><n-divider vertical />
              <a href="https://github.com/2lovecode" target="_blank">@2lovecode</a><n-divider vertical />
              <a href="https://github.com/JerryLookupU" target="_blank">@JerryLookupU</a><n-divider vertical />
            </p>
            <p>
              感谢以下开源项目：
              <a href="https://github.com/wailsapp/wails" target="_blank">Wails</a><n-divider vertical />
              <a href="https://github.com/vuejs" target="_blank">Vue</a><n-divider vertical />
              <a href="https://github.com/tusen-ai/naive-ui" target="_blank">NaiveUI</a><n-divider vertical />
            </p>
          </div>
          <n-divider title-placement="center">关于版权和技术支持申明</n-divider>
          <div style="justify-self: center;text-align: left" >
            <p style="color: #FAA04A">如有问题，请先查看项目文档和微信公众号教程，如果问题依然存在，请优先加群（491605333）咨询。</p>
            <p>
              如需软件商业授权或定制开发，请联系作者微信(备注 商业咨询)：ArvinLovegood
            </p>
            <n-divider/>
            <p>
              本软件基于开源技术构建，使用Wails、NaiveUI、Vue等开源项目。技术上如有问题，可以先向对应的开源社区请求帮助。
            </p>
            <p>
              开源不易，本人精力和时间有限，如确实需要一对一技术支持，<i style="color: crimson">请先赞助！</i>联系微信(备注 技术支持)：ArvinLovegood
            </p>
            <p style="color: #FAA04A">*加微信或者QQ时，请先备注或留言需求(如：<a href="#support">技术支持</a>，功能建议，商业咨询等，否则会被忽略)</p>
            <n-table id="support">
              <n-thead>
                <n-tr>
                  <n-th>技术支持方式</n-th><n-th>赞助(元)</n-th>
                </n-tr>
              </n-thead>
              <n-tbody>
                <n-tr>
                  <n-td>
                    加 QQ：506808970，微信：ArvinLovegood
                  </n-td>
                  <n-td>
                    100/次
                  </n-td>
                </n-tr>
                <n-tr>
                  <n-td>
                    长期技术支持（不限次数，新功能优先体验等）
                  </n-td>
                  <n-td>
                    5000
                  </n-td>
                </n-tr>
              </n-tbody>
            </n-table>
          </div>

        </n-card>

        <n-modal
          v-model:show="showManual"
          preset="card"
          title="用户手册"
          style="width: 90vw; max-height: 90vh"
          :bordered="false"
          :segmented="{ content: true, footer: true }"
        >
          <div style="display: flex; max-height: 75vh;">
            <div v-if="catalogList.length" class="manual-catalog" style="width: 240px; min-width: 240px; border-right: 1px solid var(--n-border-color); padding: 8px 4px; overflow-y: auto;">
              <div style="font-weight: bold; margin-bottom: 8px; padding: 0 8px;">目录</div>
              <n-tree
                :data="catalogTree"
                :block-line="true"
                :block-node="true"
                :selectable="true"
                :cancelable="false"
                default-expand-all
                key-field="key"
                label-field="label"
                children-field="children"
                @update:selected-keys="onTreeSelect"
              />
            </div>
            <div ref="manualScrollRef" style="flex: 1; overflow-y: auto; padding: 0 16px;">
              <MdPreview style="text-align: left;" :id="manualId" v-model="manualContent" :theme="theme" :preview-theme="'github'" :md-heading-id="slugifyHeading" @onHtmlChanged="extractCatalog" />
            </div>
          </div>
        </n-modal>
      </n-space>
</template>

<style scoped>
/* 可以在这里添加一些样式 */
h1, h2 {
  margin: 0;
  padding: 6px 0;
}

p {
  margin: 2px 0;
}

ul {
  list-style-type: disc;
  padding-left: 20px;
}

a {
  color: #18a058;
  text-decoration: none;
}

a:hover {
  text-decoration: underline;
}

.manual-catalog > div:hover {
  color: #18a058;
}

.manual-catalog :deep(.n-tree-node-content) {
  text-align: left;
  justify-content: flex-start;
}

.manual-catalog :deep(.n-tree-node) {
  text-align: left;
}
</style>
