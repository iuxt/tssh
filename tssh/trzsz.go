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
	"fmt"
	"io"
	"strings"

	"github.com/trzsz/trzsz-go/trzsz"
)

type transferOptions struct {
	enableZmodem   bool
	enableDragFile bool
	enableOSC52    bool
	disableFilter  bool
}

func isConfigNo(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "0", "false", "no", "off":
		return true
	default:
		return false
	}
}

func isConfigYes(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func getTransferOptions(args *sshArgs) transferOptions {
	enableZmodem := true
	if isConfigNo(getExOptionConfig(args, "EnableZmodem")) {
		enableZmodem = false
	}
	enableDragFile := args.DragFile || isConfigYes(getExOptionConfig(args, "EnableDragFile"))
	enableOSC52 := isConfigYes(getExOptionConfig(args, "EnableOSC52"))
	return transferOptions{
		enableZmodem:   enableZmodem,
		enableDragFile: enableDragFile,
		enableOSC52:    enableOSC52,
		disableFilter:  !enableZmodem && !enableDragFile && !enableOSC52,
	}
}

func getDragFileUploadCommand(args *sshArgs) string {
	dragFileUploadCommand := getExOptionConfig(args, "DragFileUploadCommand")
	if dragFileUploadCommand == "" {
		dragFileUploadCommand = userConfig.dragFileUploadCommand
	}
	if dragFileUploadCommand == "" {
		dragFileUploadCommand = "rz"
	}
	return dragFileUploadCommand
}

func setupTransferFilter(sshConn *sshConnection) error {
	// not terminal or not tty
	if !isTerminal || !sshConn.tty {
		return nil
	}

	args := sshConn.param.args
	options := getTransferOptions(args)

	if options.disableFilter {
		onTerminalResize(func(width, height int) {
			currentTerminalWidth.Store(int32(width))
			_ = sshConn.session.WindowChange(height, width)
		})
		return nil
	}

	clientIn, writerIn := io.Pipe()
	readerOut, clientOut := io.Pipe()
	serverIn, serverOut := sshConn.serverIn, sshConn.serverOut
	sshConn.serverIn, sshConn.serverOut = writerIn, readerOut

	trzsz.SetAffectedByWindows(false)

	width, _, err := getTerminalSize()
	if err != nil {
		return fmt.Errorf("get terminal size failed: %v", err)
	}

	// custom configuration
	defaultUploadPath := getExOptionConfig(args, "DefaultUploadPath")
	if defaultUploadPath == "" {
		defaultUploadPath = userConfig.defaultUploadPath
	}
	defaultDownloadPath := getExOptionConfig(args, "DefaultDownloadPath")
	if defaultDownloadPath == "" {
		defaultDownloadPath = userConfig.defaultDownloadPath
	}
	dragFileUploadCommand := getDragFileUploadCommand(args)

	// create a transfer filter for zmodem, drag upload, and OSC52
	//
	//   os.Stdin  ┌────────┐   os.Stdin   ┌─────────────┐   ServerIn   ┌────────┐
	// ───────────►│        ├─────────────►│             ├─────────────►│        │
	//             │        │              │ TrzszFilter │              │        │
	// ◄───────────│ Client │◄─────────────┤             │◄─────────────┤ Server │
	//   os.Stdout │        │   os.Stdout  └─────────────┘   ServerOut  │        │
	// ◄───────────│        │◄──────────────────────────────────────────┤        │
	//   os.Stderr └────────┘                  stderr                   └────────┘
	trzszFilter := trzsz.NewTrzszFilter(clientIn, clientOut, serverIn, serverOut, trzsz.TrzszOptions{
		TerminalColumns: int32(width),
		DetectDragFile:  options.enableDragFile,
		DetectTraceLog:  args.TraceLog,
		EnableZmodem:    options.enableZmodem,
		EnableOSC52:     options.enableOSC52,
		DisableTrzsz:    true,
	})

	// reset terminal and close on exit
	addOnExitFunc(func() { trzszFilter.ResetTerminal(); trzszFilter.Close() })

	// reset terminal size on resize
	onTerminalResize(func(width, height int) {
		currentTerminalWidth.Store(int32(width))
		trzszFilter.SetTerminalColumns(int32(width))
		_ = sshConn.session.WindowChange(height, width)
	})

	// setup transfer config
	trzszFilter.SetDefaultUploadPath(defaultUploadPath)
	trzszFilter.SetDefaultDownloadPath(defaultDownloadPath)
	trzszFilter.SetDragFileUploadCommand(dragFileUploadCommand)
	trzszFilter.SetProgressColorPair(userConfig.progressColorPair)

	// setup redraw screen
	trzszFilter.SetRedrawScreenFunc(sshConn.session.RedrawScreen)

	return nil
}
