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
)

func execEncodeSecret() (int, bool) {
	secret, err := readSecret("Password or secret to be encoded: ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "encode secret input failed: %v\r\n", err)
		return kExitCodeToolsError, true
	}
	if len(secret) == 0 {
		fmt.Fprintln(os.Stderr, "password or secret cannot be empty")
		return kExitCodeToolsError, true
	}
	encoded, err := encodeSecret(secret)
	if err != nil {
		fmt.Fprintf(os.Stderr, "encode secret failed: %v\r\n", err)
		return kExitCodeToolsError, true
	}
	fmt.Printf("Encoded secret for configuration: %s\r\n", encoded)
	return 0, true
}

func execLocalTools(args *sshArgs) (int, bool) {
	if args.EncSecret {
		return execEncodeSecret()
	}
	return 0, false
}
