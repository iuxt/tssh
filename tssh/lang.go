/*
MIT License

Copyright (c) 2023-2026 The Trzsz SSH Authors.

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
*/

package tssh

import (
	"strings"
)

var english = map[string]string{
	"console/title":     "Tssh Console",
	"console/send_char": "Send the escape character '{0}' ( {0} : Enter '{0}' )",
	"console/suspend":   "Suspend the current SSH process ( ^Z : Ctrl + Z )",
	"console/terminate": "Terminate the current SSH session ( . : Exit / Kill )",
	"console/notes":     "↑/↓/j/k Move • Enter Select • q Quit",
}

var chinese = map[string]string{
	"console/title":     "Tssh 控制台",
	"console/send_char": "发送转义字符 '{0}' ( {0} : 输入 '{0}' )",
	"console/suspend":   "暂停当前 SSH 进程 ( ^Z : Ctrl + Z )",
	"console/terminate": "退出当前 SSH 会话 ( . : Exit / Kill )",
	"console/notes":     "↑/↓/j/k 移动 • Enter 选择 • q 退出",
}

func getText(key string) string {
	switch strings.ToLower(userConfig.language) {
	case "english":
		return english[key]
	case "chinese":
		return chinese[key]
	default:
		return english[key]
	}
}
