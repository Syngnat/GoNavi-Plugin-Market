package market

import (
	"fmt"
	"regexp"
)

const (
	// maxArtifactIconBytes 与宿主一致：图标在未启动插件的情况下被读入内存并嗅探，
	// 因此包内文件本体也必须受限，否则市场放行的图标在客户端会被静默丢弃。
	// 5 MiB 允许作者放高分辨率品牌图；宿主读取时会再次按同一上限拒绝超限文件。
	maxArtifactIconBytes     = 5 << 20
	maxArtifactIconPathBytes = 256
)

// artifactIconPattern 只接受光栅图。宿主在主窗口渲染图标，SVG 属于可执行文档，
// 两端都必须拒绝，不能靠扩展名伪装混入。
var artifactIconPattern = regexp.MustCompile(`(?i)\.(png|jpe?g|webp|gif)$`)

// validArtifactIconPath 与宿主 validPackageIcon 同口径：包内相对路径、长度受限、
// 扩展名为光栅图。顶层 manifest.icon 与 contributes.aiProviders[].icon 共用此规则。
func validArtifactIconPath(name string) bool {
	return len(name) <= maxArtifactIconPathBytes && artifactIconPattern.MatchString(name) && safeArtifactPath(name) == nil
}

// referenceIcon 要求图标引用包内一个精确的普通文件，并套用宿主的读取上限。
func (index *artifactIndex) referenceIcon(name string) error {
	if err := index.reference(name); err != nil {
		return err
	}
	if size := index.paths[foldArtifactPath(name)].size; size > maxArtifactIconBytes {
		return fmt.Errorf("icon exceeds the %d byte limit", maxArtifactIconBytes)
	}
	return nil
}
