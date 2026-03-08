import {createMemoryHistory, createRouter, createWebHashHistory, createWebHistory} from 'vue-router'

import stockView from '../components/stock.vue'
import settingsView from '../components/settings.vue'
import aboutView from "../components/about.vue";
import fundView from "../components/fund.vue";
import marketView from "../components/market.vue";
import agentChat from "../components/agent-chat.vue"
import research from "../components/researchIndex.vue";
import KlineView from "../components/KlineView.vue";

const routes = [
    { path: '/', component: stockView,name: 'stock'},
    { path: '/fund', component: fundView,name: 'fund' },
    { path: '/settings', component: settingsView,name: 'settings' },
    { path: '/about', component: aboutView,name: 'about' },
    { path: '/market', component: marketView,name: 'market' },
    { path: '/agent', component: agentChat,name: 'agent' },
    { path: '/research', component: research,name: 'research' },
    { path: '/kline', component: KlineView, name: 'kline' },

]

const router = createRouter({
    //history: createWebHistory(),
    history: createWebHashHistory(),
    routes,
})

export default router