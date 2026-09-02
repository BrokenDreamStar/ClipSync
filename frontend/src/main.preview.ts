/* eslint-disable */
// 临时预览入口：mock 掉 Wails 运行时与 App 绑定，在纯浏览器里渲染真实 App.vue，
// 仅用于开发期在浏览器里查看界面，不属于正式构建产物。
(window as any).go = {
  main: {
    App: {
      GetConfig: () => ({
        name: 'starm-mac',
        port: 9250,
        device_id: 'a1b2c3d4e5f6',
        peers: [
          { id: 'f6e5d4c3b2a1', name: 'office-pc', addr: '192.168.1.21:9250' },
        ],
      }),
      SaveConfig: () => '',
      GetPeers: () => [
        { id: 'f6e5d4c3b2a1', name: 'office-pc', addr: '192.168.1.21:9250' },
      ],
      PairWith: () => '已发起配对请求，请等待对方确认',
      PairDiscovered: () => ({}),
      RespondPairRequest: () => '',
      GetPairRequests: () => [],
      RemovePeer: () => '',
      RemovePeerByName: () => '',
      IsAddrOnline: (a: string) => a === '192.168.1.21:9250',
      IsPairedByName: (n: string) => n === 'office-pc',
      GetDiscovered: () => [
        { device_id: 'a1b2c3d4e5f6', name: 'starm-mac', addr: '192.168.1.20:9250', last_seen: new Date(Date.now() - 30000).toISOString() },
        { device_id: 'f6e5d4c3b2a1', name: 'office-pc', addr: '192.168.1.22:9250', last_seen: new Date(Date.now() - 120000).toISOString() },
      ],
      ScanNow: () => {},
      GetConnectedPeers: () => ['office-pc'],
      GetHistory: () => [
        { id: '1', kind: 'text', from: 'starm-mac', size: 128, time: new Date(Date.now() - 5000).toISOString(), preview: '这是一条剪贴板同步的文本示例，方便在局域网内快速传递。' },
        { id: '2', kind: 'image', from: 'office-pc', size: 4032, time: new Date(Date.now() - 120000).toISOString(), preview: '' },
        { id: '3', kind: 'text', from: 'office-pc', size: 66, time: new Date(Date.now() - 3600000).toISOString(), preview: '局域网剪贴板同步，真的很方便。' },
      ],
      GetHistoryData: () => 'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+M8AAAMBAQDJ/pLvAAAAAElFTkSuQmCC',
      CopyLocal: () => '',
    },
  },
}

;(window as any).runtime = {
  EventsOnMultiple: () => {},
  EventsOn: () => {},
  EventsOff: () => {},
  EventsOffAll: () => {},
  EventsOnce: () => {},
  LogPrint: () => {},
  LogInfo: () => {},
}

import { createApp } from 'vue'
import './style.css'
import App from './App.vue'

createApp(App).mount('#app')
