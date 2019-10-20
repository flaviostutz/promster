package main

import (
	"bufio"
	"bytes"
	"crypto/sha512"
	"fmt"
	"strings"
	"text/template"
	"time"

	"github.com/go-cmd/cmd"
	"github.com/sirupsen/logrus"
)

//ShellContext container to transport a Cmd reference
type ShellContext struct {
	//CmdRef cmd.Cmd pointer that can be used to set command references that should be killed when a backup deletion of a running job is detected
	CmdRef *cmd.Cmd
}

//ExecShellTimeout execute a shell command (like bash -c 'your command') with a timeout. After that time, the process will be cancelled
func ExecShellTimeout(command string, timeout time.Duration, ctx *ShellContext) (string, error) {
	logrus.Debugf("shell command: %s", command)
	acmd := cmd.NewCmd("sh", "-c", command)
	statusChan := acmd.Start() // non-blocking
	running := true
	if ctx != nil {
		ctx.CmdRef = acmd
	}

	//kill if taking too long
	if timeout > 0 {
		logrus.Debugf("Enforcing timeout %s", timeout)
		go func() {
			startTime := time.Now()
			for running {
				if time.Since(startTime) >= timeout {
					logrus.Warnf("Stopping command execution because it is taking too long (%d seconds)", time.Since(startTime))
					acmd.Stop()
				}
				time.Sleep(1 * time.Second)
			}
		}()
	}

	// logrus.Debugf("Waiting for command to finish...")
	<-statusChan
	// logrus.Debugf("Command finished")
	running = false

	out := GetCmdOutput(acmd)
	status := acmd.Status()
	logrus.Debugf("shell output (%d): %s", status.Exit, out)
	if status.Exit != 0 {
		return out, fmt.Errorf("Failed to run command: '%s'; exit=%d; out=%s", command, status.Exit, out)
	} else {
		return out, nil
	}
}

//ExecShell execute a shell command (like bash -c 'your command')
func ExecShell(command string) (string, error) {
	return ExecShellTimeout(command, 0, nil)
}

//ExecShellf execute a shell command (like bash -c 'your command') but with format replacements
func ExecShellf(command string, args ...interface{}) (string, error) {
	cmd := fmt.Sprintf(command, args...)
	return ExecShellTimeout(cmd, 0, nil)
}

//GetCmdOutput join stdout and stderr in a single string from Cmd
func GetCmdOutput(cmd *cmd.Cmd) string {
	status := cmd.Status()
	out := strings.Join(status.Stdout, "\n")
	out = out + "\n" + strings.Join(status.Stderr, "\n")
	return out
}

func linesToArray(lines string) ([]string, error) {
	var result = []string{}
	scanner := bufio.NewScanner(strings.NewReader(lines))
	for scanner.Scan() {
		result = append(result, scanner.Text())
	}
	if scanner.Err() != nil {
		return []string{}, scanner.Err()
	}
	return result, nil
}

func reverseArray(lines []string) []string {
	for i, j := 0, len(lines)-1; i < j; i, j = i+1, j-1 {
		lines[i], lines[j] = lines[j], lines[i]
	}
	return lines
}

func trunc(str string, num int) string {
	bnoden := str
	if len(str) > num {
		if num > 3 {
			num -= 3
		}
		bnoden = str[0:num]
	}
	return bnoden
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

func unique(intSlice []string) []string {
	keys := make(map[string]bool)
	list := []string{}
	for _, entry := range intSlice {
		if _, value := keys[entry]; !value {
			keys[entry] = true
			list = append(list, entry)
		}
	}
	return list
}

func executeTemplate(dir string, templ string, input map[string]interface{}) (string, error) {
	tmpl := template.New("root")
	tmpl1, err := tmpl.ParseGlob(dir + "/*.tmpl")
	buf := new(bytes.Buffer)
	err = tmpl1.ExecuteTemplate(buf, templ, input)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

// func setValue(ctx context.Context, cli *clientv3.Client, kv clientv3.KV) {
// 	lease, _ := cli.Grant(ctx, 1)
// }
func stringSha512(str string) string {
	hashedByte := sha512.Sum512([]byte(str))
	hashedString := string(hashedByte[:])

	return hashedString
}

func hashList(list []string) []string {
	hashedList := make([]string, 0)
	for _, item := range list {
		hash := stringSha512(item)
		hashedList = append(hashedList, hash)
	}

	return hashedList
}
