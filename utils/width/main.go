package width

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

var TEST = 123
var stty_exec_count uint64 = 0
var last_stty_exec = time.Now()
var min_stty_exec_dur = time.Duration(750 * time.Millisecond)
var cached_out = []byte{}

func size() (string, error) {
	s := time.Now()
	msg := ``
	if time.Since(last_stty_exec) < min_stty_exec_dur && len(cached_out) > 0 {
		msg = fmt.Sprintf("skipping stty exec......")
		if false {
			fmt.Fprintf(os.Stderr, "%s\n", msg)
		}
		return string(cached_out), nil
	} else {
		msg = fmt.Sprintf("not skipping stty exec......")
		if false {
			fmt.Fprintf(os.Stderr, "%s\n", msg)
		}
	}
	cmd := exec.Command("stty", "size")
	cmd.Stdin = os.Stdin

	out, err := cmd.Output()
	dur := time.Since(s)
	msg = fmt.Sprintf("Completed stty #%d in %s. Last stty exec was %s ago.", stty_exec_count, dur, time.Since(last_stty_exec))
	atomic.AddUint64(&stty_exec_count, 1)
	fmt.Fprintf(os.Stderr, "%s\n", msg)
	if err == nil {
		cached_out = out
		last_stty_exec = time.Now()
	}
	return string(out), err
}

func parse(input string) (uint, uint, error) {
	parts := strings.Split(input, " ")

	x, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, err
	}

	y, err := strconv.Atoi(strings.Replace(parts[1], "\n", "", 1))
	if err != nil {
		return 0, 0, err
	}

	return uint(x), uint(y), nil
}

// Dimensions returns the width and height of the terminal.
func Dimensions() (uint, uint, error) {
	output, err := size()
	if err != nil {
		return 0, 0, err
	}

	height, width, err := parse(output)
	if err != nil {
		return 0, 0, err
	}

	return width, height, nil
}

// Width returns the width of the terminal.
func Width() (uint, error) {
	output, err := size()
	if err != nil {
		return 0, err
	}

	_, width, err := parse(output)

	return width, err
}

// Height returns the height of the terminal.
func Height() (uint, error) {
	output, err := size()
	if err != nil {
		return 0, err
	}

	height, _, err := parse(output)

	return height, err
}
