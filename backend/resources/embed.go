// 嵌入静态资源，供其他包引用
package resources

import _ "embed"

//go:embed instructions.md
var defaultInstructions string

//go:embed instructions-simple.md
var simpleInstructions string

// DefaultInstructions 是完整版本的系统提示词。
var DefaultInstructions = defaultInstructions

// SimpleInstructions 是当前默认使用的精简版本系统提示词。
var SimpleInstructions = simpleInstructions

// Instructions 是当前使用的系统提示词，切换时只需修改此处
var Instructions = SimpleInstructions
