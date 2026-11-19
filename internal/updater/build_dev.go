//go:build !production

package updater

// devBuild：非 production 标签构建（wails3 dev）。
// 各平台 Taskfile 里正式构建都带 -tags production，开发构建不带；
// 更新器据此拒绝自更新（开发二进制在 build/bin 下，不该被 Release 产物替换）。
const devBuild = true
