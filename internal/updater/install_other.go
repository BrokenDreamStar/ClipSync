//go:build !(darwin || linux || windows)

package updater

// 其他平台（发布矩阵未覆盖）的桩实现：检查与安装都明确报不支持。
// 新增平台安装器时必须同步维护这里（见 AGENTS.md「已知坑」）。

func platformSupported() error { return errUnsupportedInstall }
func platformInstall(string, *Asset, *Manifest) (relaunchSpec, error) {
	return relaunchSpec{}, errUnsupportedInstall
}
func verifyAssetHeader(string) error { return nil }
func platformCleanup()               {}

// spawnRelaunch 的桩：正常流程走不到（platformInstall 先拒绝），
// 补上只为让该文件覆盖的组合编译完整。
func spawnRelaunch(relaunchSpec) error { return errUnsupportedInstall }
