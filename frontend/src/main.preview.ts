/* eslint-disable */
// 临时预览入口：mock 掉 Wails 传输层与 App 绑定，在纯浏览器里渲染真实 App.vue，
// 仅用于开发期在浏览器里查看界面，不属于正式构建产物。
// v3 绑定统一走 @wailsio/runtime 的 Call 对象，这里用官方 setTransport
// 钩子按方法名路由到 mock 数据，替代 v2 时代对 window.go 的整体伪造。
import { setTransport, objectNames } from '@wailsio/runtime'

type MockFn = (...args: any[]) => any

const iso = (offsetMs: number) => new Date(Date.now() - offsetMs).toISOString()

const peersMock = [
  { id: 'f6e5d4c3b2a1', name: 'office-pc', addr: '192.168.1.21:9250' },
]

const historyMock = [
  { id: '1', kind: 'text', from: 'starm-mac', size: 128, time: iso(5000), preview: '这是一条剪贴板同步的文本示例，方便在局域网内快速传递。' },
  { id: '2', kind: 'image', from: 'office-pc', size: 4032, time: iso(120000), preview: '' },
  { id: '3', kind: 'text', from: 'office-pc', size: 66, time: iso(3600000), preview: '局域网剪贴板同步，真的很方便。' },
]

// 键格式为 '包名.结构体.方法'，与 -names 模式生成的绑定调用一致。
const appMocks: Record<string, MockFn> = {
  'main.App.GetConfig': () => [
    {
      name: 'starm-mac',
      port: 9250,
      device_id: 'a1b2c3d4e5f6',
      theme: 'system',
      start_minimized: false,
      peers: peersMock,
    },
    '',
  ],
  'main.App.SaveConfig': () => '',
  'main.App.GetPeers': () => peersMock,
  'main.App.PairWith': () => [null, '已发起配对请求，请等待对方确认'],
  'main.App.PairDiscovered': () => [peersMock[0], ''],
  'main.App.RespondPairRequest': () => '',
  'main.App.GetPairRequests': () => [],
  'main.App.RemovePeer': () => '',
  'main.App.RemovePeerByName': () => '',
  'main.App.IsAddrOnline': (addr: string) => addr === '192.168.1.21:9250',
  'main.App.IsPairedByName': (name: string) => name === 'office-pc',
  'main.App.GetDiscovered': () => [
    { device_id: 'a1b2c3d4e5f6', name: 'starm-mac', addr: '192.168.1.20:9250', last_seen: iso(30000) },
    { device_id: 'f6e5d4c3b2a1', name: 'office-pc', addr: '192.168.1.22:9250', last_seen: iso(120000) },
  ],
  'main.App.ScanNow': () => undefined,
  'main.App.GetConnectedPeers': () => ['office-pc'],
  'main.App.GetHistory': () => [historyMock, ''],
  'main.App.GetHistoryData': () => [
    'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+M8AAAMBAQDJ/pLvAAAAAElFTkSuQmCC',
    '',
  ],
  'main.App.CopyLocal': () => '',
  'main.App.SetTheme': () => '',
  'main.App.GetAutostart': () => false,
  'main.App.SetAutostart': () => '',
  'main.App.SetStartMinimized': () => '',
  'main.App.GetAppVersion': () => ['0.1.0', ''],
  // 预览默认展示「有新版本」形态，方便查看更新 UI；把 hasUpdate 置 false
  // 即可看「已是最新版本」形态。
  'main.App.CheckUpdate': () => [
    {
      current: { version: '0.1.0', platform: 'macos-universal', supported: true },
      latest: {
        tag: 'v0.0.5',
        version: '0.0.5',
        publishedAt: iso(86400000),
        htmlUrl: 'https://github.com/BrokenDreamStar/ClipSync/releases/tag/v0.0.5',
        assetName: 'ClipSync-0.0.5-macos-universal.dmg',
        assetSize: 28_000_000,
      },
      hasUpdate: true,
      error: null,
      checkedAt: Date.now(),
    },
    '',
  ],
  'main.App.GetUpdateState': () => [{ phase: 'idle', tag: null, received: 0, total: null, error: null }, ''],
  'main.App.ApplyUpdate': () => [
    { phase: 'downloading', tag: 'v0.0.5', received: 0, total: 28000000, error: null },
    '',
  ],
  'main.App.Quit': () => undefined,
  'main.App.ShowMainWindow': () => undefined,
}

setTransport({
  call(objectID, _method, _windowName, args) {
    // 绑定调用：objectID 为 Call 对象，args 形如 { methodName, args, 'call-id' }。
    if (objectID === objectNames.Call) {
      const fn = appMocks[args?.methodName as string]
      return Promise.resolve(fn ? fn(...(args?.args ?? [])) : null)
    }
    return Promise.resolve(null)
  },
})

import { createApp } from 'vue'
import './style.css'
import App from './App.vue'

createApp(App).mount('#app')
