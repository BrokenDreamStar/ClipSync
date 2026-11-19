package main

import (
	"regexp"
	"testing"
)

func TestAppVersionFromConfig(t *testing.T) {
	// 内嵌的 build/config.yml 必须能解析出三段式版本号，更新器依赖它比较版本。
	// 不做精确值断言（那会变成第三个版本同步点），只保证格式可用。
	v := appVersionFromConfig(buildConfig)
	if !regexp.MustCompile(`^\d+\.\d+\.\d+$`).MatchString(v) {
		t.Fatalf("内嵌 build/config.yml 解析出的版本号非法：%q", v)
	}
}
