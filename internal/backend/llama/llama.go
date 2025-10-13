package llama

import (
	"bufio"
	"io"
	"os/exec"
	"sync"
)

type Backend struct {
	modelPath string
	cliPath   string
	mu        *sync.Mutex
	cmd       *exec.Cmd
	stdin     io.WriteCloser
	scanner   *bufio.Scanner
	loaded    bool
}
