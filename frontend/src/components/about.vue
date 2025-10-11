<script setup>
// import { MdPreview } from 'md-editor-v3';
// preview.css相比style.css少了编辑器那部分样式
import 'md-editor-v3/lib/preview.css';
import {h, onBeforeUnmount, onMounted, ref} from 'vue';
import {CheckUpdate, GetVersionInfo,GetSponsorInfo,OpenURL} from "../../wailsjs/go/main/App";
import {EventsOff, EventsOn,Environment} from "../../wailsjs/runtime";
import {NAvatar, NButton, useNotification} from "naive-ui";
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

onMounted(() => {
  document.title = '关于软件';
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
    })

  });



})
onBeforeUnmount(() => {
  notify.destroyAll()
  EventsOff("updateVersion")
})

EventsOn("updateVersion",async (msg) => {
  const githubTimeStr = msg.published_at;
  // 创建一个 Date 对象
  const utcDate = new Date(githubTimeStr);
// 获取本地时间
  const date = new Date(utcDate.getTime());
  const year = date.getFullYear();
// getMonth 返回值是 0 - 11，所以要加 1
  const month = String(date.getMonth() + 1).padStart(2, '0');
  const day = String(date.getDate()).padStart(2, '0');
  const hours = String(date.getHours()).padStart(2, '0');
  const minutes = String(date.getMinutes()).padStart(2, '0');
  const seconds = String(date.getSeconds()).padStart(2, '0');

  const formattedDate = `${year}-${month}-${day} ${hours}:${minutes}:${seconds}`;

  //console.log("GitHub UTC 时间:", utcDate);
  //console.log("转换后的本地时间:", formattedDate);
  notify.info({
    avatar: () =>
        h(NAvatar, {
          size: 'small',
          round: false,
          src: icon.value
        }),
    title: '发现新版本: ' + msg.tag_name,
    content: () => {
      //return h(MdPreview, {theme:'dark',modelValue:msg.commit?.message}, null)
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

</script>

<template>
      <n-space vertical size="large"  style="--wails-draggable:no-drag">
        <!-- Software description -->
        <n-card size="large">
          <n-divider title-placement="center">About Software</n-divider>
          <n-space vertical >
            <n-image width="100" :src="icon" />
            <h1>
              <n-badge v-if="!vipLevel"  :value="versionInfo" :offset="[50,10]"  type="success">
                <n-gradient-text type="info" :size="50" >go-stock</n-gradient-text>
              </n-badge>
              <n-badge v-if="vipLevel"  :value="versionInfo" :offset="[50,10]"  type="success">
                <n-gradient-text type="warning" :size="50" >go-stock</n-gradient-text><n-tag :bordered="false" size="small" type="warning">VIP{{vipLevel}}</n-tag>
              </n-badge>
            </h1>
            <n-gradient-text type="warning"  v-if="vipLevel" >VIP expiration: {{vipEndTime}}</n-gradient-text>
            <n-button size="tiny" @click="CheckUpdate(1)"  type="info" tertiary >Check for Updates</n-button>
            <div style="justify-self: center;text-align: left" >
              <p>Real-time watchlist monitoring, AI-powered stock analysis tool built with Wails and NaiveUI</p>
              <p>Currently supports A-shares, Hong Kong stocks, US stocks. Future plans include funds, ETFs, etc.</p>
              <p>Supports DeepSeek, OpenAI, Ollama, LMStudio, AnythingLLM, <a href="https://cloud.siliconflow.cn/i/foufCerk" target="_blank">SiliconFlow</a>, <a href="https://www.volcengine.com/experience/ark?utm_term=202502dsinvite&ac=DSASUQY5&rc=IJSE43PZ" target="_blank">Volcano Ark</a>, Alibaba Cloud Bailian and other platforms or models</p>
              <p>
                <i style="color: crimson">This software is for learning and research purposes only. AI analysis results are for reference only. This software does not provide any investment advice or decisions. Use at your own risk!</i>
              </p>
              <p>
                Welcome to star on GitHub: <a href="https://github.com/ArvinLovegood/go-stock" target="_blank">go-stock</a><n-divider vertical />
                <a href="https://github.com/ArvinLovegood/go-stock" target="_blank">GitHub</a><n-divider vertical />
                <a href="https://github.com/ArvinLovegood/go-stock/issues" target="_blank">Issues</a><n-divider vertical />
                <a href="https://github.com/ArvinLovegood/go-stock/releases" target="_blank">Releases</a><n-divider vertical />
              </p>
              <p v-if="updateLog">Update Notes: {{updateLog}}</p>
              <p>Project Community: <a href="https://go-stock.sparkmemory.top/" target="_blank">https://go-stock.sparkmemory.top/</a></p>
              <p>QQ Group: <a href="http://qm.qq.com/cgi-bin/qm/qr?_wv=1027&k=0YQ8qD3exahsD4YLNhzQTWe5ssstWC89&authKey=usOMMRFtIQDC%2FYcatHYapcxQbJ7PwXPHK9OypTXWzNjAq%2FRVvQu9bj2lRgb%2BSZ3p&noverify=0&group_code=491605333" target="_blank">491605333</a></p>
            </div>
          </n-space>
          <n-divider title-placement="center">Support💕Open Source</n-divider>
          <n-flex justify="center">
            <n-table  size="small" style="width: 820px">
              <n-thead>
                <n-tr>
                  <n-th>Sponsorship Plan</n-th>
                  <n-th>Sponsorship Level</n-th>
                  <n-th>Benefits</n-th>
                </n-tr>
              </n-thead>
              <n-tbody>
                <n-tr>
                  <n-td>Monthly 0 RMB</n-td><n-td>vip0</n-td><n-td>🌟 All features, automatic software updates (download from GitHub), solve GitHub platform network issues yourself.</n-td>
                </n-tr>
                <n-tr>
                  <n-td>Sponsor 18.8 RMB/month<br>Sponsor 120 RMB/year</n-td><n-td>vip1</n-td><n-td>💕 All features, automatic software updates (download from CDN), fast and convenient updates. AI configuration guidance, prompt reference, etc.</n-td>
                </n-tr>
                <n-tr>
                  <n-td>Sponsor 28.8 RMB/month<br>Sponsor 240 RMB/year</n-td><n-td>vip2</n-td><n-td>💕 All vip1 features, free SiliconFlow AI analysis service💕</n-td>
                </n-tr>
                <n-tr>
                  <n-td>Monthly sponsor X RMB</n-td><n-td>vipX</n-td><n-td>🧩 More plans, depending on the development of the go-stock open source project...(Accepting GitHub project README advertising💖)</n-td>
                </n-tr>
              </n-tbody>
            </n-table>
          </n-flex>
          <n-divider title-placement="center">About Author</n-divider>
          <n-space vertical>
<!--            <h1>About Author</h1>-->
            <n-avatar width="100" src="https://avatars.githubusercontent.com/u/7401917?v=4" />
            <h2><a href="https://github.com/ArvinLovegood" target="_blank">@ArvinLovegood</a></h2>
            <p>A programming enthusiast, welcome to follow my Github/WeChat Official Account</p>
            <n-image width="300" :src="wxgzh" />
            <p>Open source is not easy, if you find it useful, you can buy the author a coffee.</p>
            <n-flex justify="center">
              <n-image width="200" :src="alipay" />
              <n-image width="200" :src="wxpay" />
            </n-flex>
          </n-space>
          <n-divider title-placement="center">Acknowledgements</n-divider>
          <div style="justify-self: center;text-align: left" >
            <p>
              Thanks to the following donors:
              <n-gradient-text size="small" type="warning">*晨</n-gradient-text><n-divider vertical />
            </p>
            <p>
              Thanks to the following developers:
              <a href="https://github.com/GiCo001" target="_blank">@Gico</a><n-divider vertical />
              <a href="https://github.com/CodeNoobLH" target="_blank">浓睡不消残酒</a><n-divider vertical />
              <a href="https://github.com/gnim2600" target="_blank">@gnim2600</a><n-divider vertical />
              <a href="https://github.com/XXXiaohuayanGGG" target="_blank">@XXXiaohuayanGGG</a><n-divider vertical />
              <a href="https://github.com/2lovecode" target="_blank">@2lovecode</a><n-divider vertical />
              <a href="https://github.com/JerryLookupU" target="_blank">@JerryLookupU</a><n-divider vertical />
            </p>
            <p>
              Thanks to the following open source projects:
              <a href="https://github.com/wailsapp/wails" target="_blank">Wails</a><n-divider vertical />
              <a href="https://github.com/vuejs" target="_blank">Vue</a><n-divider vertical />
              <a href="https://github.com/tusen-ai/naive-ui" target="_blank">NaiveUI</a><n-divider vertical />
            </p>
          </div>
          <n-divider title-placement="center">Copyright and Technical Support Statement</n-divider>
          <div style="justify-self: center;text-align: left" >
            <p style="color: #FAA04A">If you have any questions, please check the project documentation first. If the problem persists, please join the group (491605333) for consultation.</p>
            <p>
              For commercial licensing or custom development, please contact the author on WeChat (note: Commercial Inquiry): ArvinLovegood
            </p>
            <n-divider/>
            <p>
              This software is built on open source technology, using Wails, NaiveUI, Vue and other open source projects. For technical issues, you can first seek help from the corresponding open source community.
            </p>
            <p>
              Open source is not easy, and I have limited energy and time. If you really need one-on-one technical support, <i style="color: crimson">please sponsor first!</i> Contact WeChat (note: Technical Support): ArvinLovegood
            </p>
            <p style="color: #FAA04A">*When adding WeChat or QQ, please note or leave a message about your needs (such as: <a href="#support">Technical Support</a>, Feature Suggestions, Commercial Inquiry, etc., otherwise it will be ignored)</p>
            <n-table id="support">
              <n-thead>
                <n-tr>
                  <n-th>Technical Support Method</n-th><n-th>Sponsorship (RMB)</n-th>
                </n-tr>
              </n-thead>
              <n-tbody>
                <n-tr>
                  <n-td>
                    Add QQ: 506808970, WeChat: ArvinLovegood
                  </n-td>
                  <n-td>
                    100/time
                  </n-td>
                </n-tr>
                <n-tr>
                  <n-td>
                    Long-term technical support (unlimited times, early access to new features, etc.)
                  </n-td>
                  <n-td>
                    5000
                  </n-td>
                </n-tr>
              </n-tbody>
            </n-table>
          </div>

        </n-card>
      </n-space>
</template>

<style scoped>
/* You can add some styles here */
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
</style>
