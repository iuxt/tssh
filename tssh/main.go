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
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"github.com/mattn/go-isatty"
	"github.com/trzsz/go-arg"
	"golang.org/x/crypto/ssh"
)

func setTerminalTitle(title string) {
	fmt.Fprintf(os.Stderr, "\033]0;%s\007", title)
}

func background(args *sshArgs, dest string) (bool, error) {
	if v := os.Getenv("TRZSZ-SSH-BACKGROUND"); v == "TRUE" {
		return false, nil
	}

	newArgs, err := replaceOrAppendDest(os.Args, args.Destination, dest)
	if err != nil {
		return true, err
	}
	exePath := getExePath(newArgs[0])

	cmd := exec.Command(exePath, newArgs[1:]...)
	cmd.Args = newArgs
	cmd.Env = append(os.Environ(), "TRZSZ-SSH-BACKGROUND=TRUE")
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return true, fmt.Errorf("run in background failed: %v", err)
	}
	return true, nil
}

func getExePath(defaultPath string) string {
	if path, err := os.Executable(); err == nil {
		return path
	}
	return defaultPath
}

// replaceOrAppendDest returns a new args slice where the destination is replaced or appended.
// Returns an error if it cannot safely replace the existing destination.
func replaceOrAppendDest(args []string, oldDest, newDest string) ([]string, error) {
	newArgs := make([]string, len(args))
	copy(newArgs, args)

	if oldDest == "" {
		// No existing destination, append the new one
		newArgs = append(newArgs, newDest)
	} else if oldDest != newDest {
		// Replace old destination
		idx := -1
		count := 0
		for i, arg := range newArgs {
			if arg == oldDest {
				idx = i
				count++
			}
		}
		if count != 1 {
			return nil, fmt.Errorf("don't know how to replace the destination: %s => %s", oldDest, newDest)
		}
		newArgs[idx] = newDest
	}

	return newArgs, nil
}

var onExitFuncs []func()
var onExitMutex sync.Mutex

func cleanupOnExit() {
	onExitMutex.Lock()
	defer onExitMutex.Unlock()
	for i := len(onExitFuncs) - 1; i >= 0; i-- { // close proxy clients in order
		onExitFuncs[i]()
	}
	onExitFuncs = nil
}

func addOnExitFunc(f func()) {
	onExitMutex.Lock()
	defer onExitMutex.Unlock()
	onExitFuncs = append(onExitFuncs, f)
}

var onCloseFuncs []func()
var onCloseMutex sync.Mutex

func cleanupOnClose() {
	onCloseMutex.Lock()
	defer onCloseMutex.Unlock()
	for i := len(onCloseFuncs) - 1; i >= 0; i-- {
		onCloseFuncs[i]()
	}
	onCloseFuncs = nil
}

func addOnCloseFunc(f func()) {
	onCloseMutex.Lock()
	defer onCloseMutex.Unlock()
	onCloseFuncs = append(onCloseFuncs, f)
}

var isTerminal bool = isatty.IsTerminal(os.Stdin.Fd()) || isatty.IsCygwinTerminal(os.Stdin.Fd())

// TrzMain is the main function of tssh program.
func TsshMain(argv []string) int {
	// parse ssh args
	var args sshArgs
	parser, err := arg.NewParser(arg.Config{HideLongOptions: true, Out: os.Stderr, Exit: os.Exit}, &args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "new arg parser failed: %v\r\n", err)
		return kExitCodeArgsInvalid
	}

	if err := parser.Parse(argv); err != nil {
		if err == arg.ErrHelp {
			parser.WriteHelp(os.Stdout)
			return 0
		}
		if err == arg.ErrVersion {
			return printVersionShort()
		}
		parser.WriteUsage(os.Stderr)
		fmt.Fprintf(os.Stderr, "error: %v\r\n", err)
		return kExitCodeArgsInvalid
	}

	if args.VerDetailed {
		return printVersionDetailed()
	}

	// debug log
	if args.Debug {
		enableDebugLogging = true
		debug("tssh version: %s", getTsshVersion())
	}

	// cleanup on exit
	defer cleanupOnExit()

	// print message after stdin reset
	defer func() {
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\r\n", err)
		}
	}()

	// init user config
	if err = initUserConfig(args.ConfigFile); err != nil {
		return kExitCodeUserConfig
	}

	// setup virtual terminal on Windows
	if isTerminal {
		if err = setupVirtualTerminal(); err != nil {
			return kExitCodeSetupWinVT
		}
	}

	// choose ssh alias
	dest := ""
	quit := false
	if args.Destination == "" || args.Destination == "FAKE_DEST_IN_WARP" {
		if !isTerminal {
			fmt.Fprintln(os.Stderr, "destination is required when running tssh in non-interactive mode")
			return kExitCodeNoDestHost
		}
		dest, quit, err = chooseAlias("")
	} else {
		dest, quit, err = predictDestination(args.Destination)
	}
	if quit {
		err = nil
		return 0
	}
	if err != nil || dest == "" {
		if err == nil {
			err = fmt.Errorf("missing destination host")
		}
		return kExitCodeNoDestHost
	}

	// run as background
	if args.Background {
		var parent bool
		parent, err = background(&args, dest)
		if err != nil {
			return kExitCodeBackground
		}
		if parent {
			return 0
		}
	}

	// start ssh program
	args.Destination = dest
	args.originalDest = dest
	var code int
	code, err = sshStart(&args)
	return code
}

func sshStart(args *sshArgs) (int, error) {
	// ssh login
	sshConn, err := sshConnect(args)
	if err != nil {
		return kExitCodeLoginFailed, err
	}
	defer func() {
		cleanupOnClose()
		sshConn.Close()
	}()

	// execute local command if necessary
	execLocalCommand(sshConn.param)

	// handle signals
	handleExitSignals(sshConn)

	// stdio forward
	if args.StdioForward != "" {
		if err = stdioForward(args, sshConn.client, args.StdioForward); err != nil {
			return kExitCodeIoFwFailed, err
		}
		return 0, nil
	}

	// request subsystem
	if args.Subsystem {
		if err = subsystemForward(sshConn.client, sshConn.cmd); err != nil {
			return kExitCodeSubFwFailed, err
		}
		return 0, nil
	}

	// ssh port forwarding
	if !sshConn.param.control {
		sshPortForward(sshConn)
	}

	// not executing remote command
	if args.NoCommand {
		_ = sshConn.client.Wait()
		return 0, nil
	}

	// open ssh session
	if err = openSession(sshConn); err != nil {
		return kExitCodeOpenSession, err
	}

	// ssh agent forward
	if !sshConn.param.control {
		sshAgentForward(sshConn)
	}

	// x11 forward
	sshX11Forward(sshConn)

	// set terminal title
	switch strings.ToLower(getExOptionConfig(args, "SetTerminalTitle")) {
	case "yes", "true":
		setTerminalTitle(args.Destination)
	}

	// enable waypipe
	if err := enableWaypipe(sshConn); err != nil {
		warning("waypipe may not be working properly: %v", err)
	}

	// run command or start shell
	if sshConn.cmd != "" {
		if err := sshConn.session.Start(sshConn.cmd); err != nil {
			return kExitCodeStartFailed, fmt.Errorf("start command [%s] failed: %v", sshConn.cmd, err)
		}
	} else {
		if err := sshConn.session.Shell(); err != nil {
			return kExitCodeShellFailed, fmt.Errorf("start shell failed: %v", err)
		}
	}

	// execute expect interactions if necessary
	execExpectInteractions(sshConn)

	// make stdin raw
	if isTerminal && sshConn.tty {
		state, err := makeStdinRaw()
		if err != nil {
			return kExitCodeStdinFailed, err
		}
		addOnExitFunc(func() { resetStdin(state) })
		defer resetStdin(state)
	}

	// setup transfer filter if necessary
	if err := setupTransferFilter(sshConn); err != nil {
		return kExitCodeTransferFilterFailed, err
	}

	// forward standard input output
	forwardStdio(sshConn)

	// cleanup and wait for exit
	code := sshConn.waitUntilExit()
	if args.Background {
		_ = sshConn.client.Wait()
	}

	// wait for the output
	outputWaitGroup.Wait()
	debug("ssh session output wait completed")
	return code, nil
}

func execLocalCommand(param *sshParam) {
	if strings.ToLower(getOptionConfig(param.args, "PermitLocalCommand")) != "yes" {
		return
	}
	localCmd := getOptionConfig(param.args, "LocalCommand")
	if localCmd == "" {
		return
	}
	expandedCmd, err := expandTokens(localCmd, param, "%CdfHhIijKkLlnprTtu")
	if err != nil {
		warning("expand LocalCommand [%s] failed: %v", localCmd, err)
		return
	}
	resolvedCmd := resolveHomeDir(expandedCmd)
	debug("exec local command: %s", resolvedCmd)

	argv, err := splitCommandLine(resolvedCmd)
	if err != nil || len(argv) == 0 {
		warning("split local command [%s] failed: %v", resolvedCmd, err)
		return
	}
	if enableDebugLogging {
		for i, arg := range argv {
			debug("local command argv[%d] = %s", i, arg)
		}
	}
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		warning("exec local command [%s] failed: %v", resolvedCmd, err)
	}
}

func openSession(sshConn *sshConnection) (err error) {
	// new session
	sshConn.session, err = sshConn.client.NewSession()
	if err != nil {
		return fmt.Errorf("new session for [%s] failed: %v", sshConn.param.args.Destination, err)
	}

	// session input and output
	sshConn.serverIn, err = sshConn.session.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe for [%s] failed: %v", sshConn.param.args.Destination, err)
	}
	sshConn.serverOut, err = sshConn.session.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe for [%s] failed: %v", sshConn.param.args.Destination, err)
	}
	sshConn.serverErr, err = sshConn.session.StderrPipe()
	if err != nil {
		return fmt.Errorf("stderr pipe for [%s] failed: %v", sshConn.param.args.Destination, err)
	}

	// send and set env
	term, err := sendAndSetEnv(sshConn.param.args, sshConn.session)
	if err != nil {
		return err
	}

	// pty is not needed if not tty in terminal
	if !isTerminal || !sshConn.tty {
		return nil
	}

	// request pty
	width, height, err := getTerminalSize()
	if err != nil {
		return fmt.Errorf("get terminal size for [%s] failed: %v", sshConn.param.args.Destination, err)
	}
	if err := sshConn.session.RequestPty(term, height, width, ssh.TerminalModes{}); err != nil {
		return fmt.Errorf("request pty for [%s] failed: %v", sshConn.param.args.Destination, err)
	}

	return nil
}

func handleExitSignals(sshConn *sshConnection) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan,
		syscall.SIGTERM, // Default signal for the kill command
		syscall.SIGHUP,  // Terminal closed (System reboot/shutdown)
		os.Interrupt,    // Ctrl+C signal
	)
	go func() {
		for sig := range sigChan {
			if enableDebugLogging && debugCleanuped.Load() {
				_, _ = os.Stderr.WriteString("\r\n")
				os.Exit(kExitCodeSignalKill)
			}
			if isRunningOnOldWindows.Load() && sig.String() == "interrupt" {
				continue
			}
			sshConn.forceExit(kExitCodeSignalKill, fmt.Sprintf("Exit due to signal [%v] from the operating system", sig))
			break
		}
	}()
}
