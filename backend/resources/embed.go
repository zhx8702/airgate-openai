// 嵌入静态资源，供其他包引用
package resources

import _ "embed"

//go:embed instructions.md
var DefaultInstructions string
