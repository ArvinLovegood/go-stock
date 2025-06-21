<script setup>
// import { MdPreview } from 'md-editor-v3';
// preview.css相比style.css少了编辑器那部分样式
import 'md-editor-v3/lib/preview.css';
import {h, onBeforeUnmount, onMounted, ref} from 'vue';
import {CheckUpdate, GetVersionInfo} from "../../wailsjs/go/main/App";
import {EventsOff, EventsOn} from "../../wailsjs/runtime";
import {NAvatar, NButton, useNotification} from "naive-ui";
const updateLog = ref('');
const versionInfo = ref('');
const icon = ref('https://raw.githubusercontent.com/ArvinLovegood/go-stock/master/build/appicon.png');
const alipay =ref('https://github.com/ArvinLovegood/go-stock/raw/master/build/screenshot/alipay.jpg')
const wxpay =ref('https://github.com/ArvinLovegood/go-stock/raw/master/build/screenshot/wxpay.jpg')
const notify = useNotification()

onMounted(() => {
  document.title = '关于软件';
  GetVersionInfo().then((res) => {
    updateLog.value = res.content;
    versionInfo.value = res.version;
    icon.value = res.icon;
    alipay.value=res.alipay;
    wxpay.value=res.wxpay;
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
          window.open(msg.html_url)
        }
      }, { default: () => '查看' })
    }
  })
})

</script>

<template>
      <n-space vertical size="large">
        <!-- 软件描述 -->
        <n-card size="large">
          <n-divider title-placement="center">关于软件</n-divider>
          <n-space vertical >
            <n-image width="100" :src="icon" />
            <h1>
              <n-badge :value="versionInfo" :offset="[50,10]"  type="success">
                <n-gradient-text type="info" :size="50" >go-stock</n-gradient-text>
              </n-badge>
            </h1>
            <n-button size="tiny" @click="CheckUpdate"  type="info" tertiary >检查更新</n-button>



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
          <n-divider title-placement="center">关于作者</n-divider>
          <n-space vertical>
<!--            <h1>关于作者</h1>-->
            <n-avatar width="100" src="https://avatars.githubusercontent.com/u/7401917?v=4" />
            <h2><a href="https://github.com/ArvinLovegood" target="_blank">@ArvinLovegood</a></h2>
            <p>一个热爱编程的小白，欢迎关注我的Github</p>
            <n-image width="300" src="https://go-stock.sparkmemory.top/assets/%E6%89%AB%E7%A0%81_%E6%90%9C%E7%B4%A2%E8%81%94%E5%90%88%E4%BC%A0%E6%92%AD%E6%A0%B7%E5%BC%8F-%E7%99%BD%E8%89%B2%E7%89%88-DEJtWc_y.png" />

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
            <p style="color: #FAA04A">如有问题，请先查看项目文档，如果问题依然存在，请优先加群（491605333）咨询。</p>
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
</style>
